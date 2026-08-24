// Command cavitation 船舶螺旋桨空化声纹判定服务入口。
//
// 支持三个标志：
//   - --addr :8080      监听地址（默认 :8080）
//   - --db ./cavitation.db  SQLite 数据库路径（默认 ./cavitation.db）
//   - --smoke-test      执行端到端冒烟：真实创建数据、关闭并重开数据库
//     验证持久化与重启恢复，随后以 0 退出码结束。
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"

	"task220-cavitation/internal/httpapi"
	"task220-cavitation/internal/model"
	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "./cavitation.db", "SQLite database path")
	smoke := flag.Bool("smoke-test", false, "run end-to-end smoke test and exit")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			fmt.Fprintln(os.Stderr, "SMOKE TEST FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("SMOKE TEST PASSED")
		return
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	app, err := service.New(db)
	if err != nil {
		log.Fatalf("init services: %v", err)
	}
	srv := httpapi.New(app)
	log.Printf("task220-cavitation listening on %s (db=%s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

// 冒烟测试的波形参数。
const (
	smokeSampleRate = 2000.0 // Hz
	smokeWindowMs   = 200    // 每窗口时长
	smokeFundFreq   = 100.0  // 螺旋桨叶频（Hz）
	smokeChannels   = 3
	smokeWindows    = 20
)

// genWindow 生成一个窗口的声纹抽样：基频 + 谐波 + 宽带噪声。
// cavitating=true 时空化泡噪声填充谐波间隙，谐波缺口比显著上升。
func genWindow(rng *rand.Rand, cavitating bool) []float64 {
	n := int(smokeSampleRate * smokeWindowMs / 1000.0)
	samples := make([]float64, n)
	noiseScale := 0.04
	if cavitating {
		noiseScale = 0.45
	}
	for i := 0; i < n; i++ {
		t := float64(i) / smokeSampleRate
		v := math.Sin(2*math.Pi*smokeFundFreq*t)
		for k := 2; k <= 6; k++ {
			v += (0.35 / float64(k)) * math.Sin(2*math.Pi*smokeFundFreq*float64(k)*t)
		}
		v += rng.NormFloat64() * noiseScale
		samples[i] = v
	}
	return samples
}

// runSmokeTest 执行端到端冒烟：
//  1. 打开数据库 A，走完整闭环：试验 -> 采集 -> 多通道片段 -> 校准 ->
//     标记噪声 -> 分析 -> 空化事件 -> 确认 -> 发布结论包（封存）；
//  2. 幂等验证：重复接收相同片段被跳过；
//  3. 封存后拒绝再接收片段；
//  4. 关闭数据库 A，重开同一路径数据库 B，验证数据仍在（重启恢复）。
func runSmokeTest(dbPath string) error {
	if dbPath != ":memory:" {
		_ = os.Remove(dbPath)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	app, err := service.New(db)
	if err != nil {
		db.Close()
		return fmt.Errorf("init services: %w", err)
	}

	rng := rand.New(rand.NewSource(42))

	// --- 步骤 1：创建试验并开始采集 ---
	trial, err := app.Trials.Create("螺旋桨空化试验 A", "1500rpm / 200kPa 多通道声纹采集", 1500, 200, 0)
	if err != nil {
		db.Close()
		return fmt.Errorf("create trial: %w", err)
	}
	trial, err = app.Trials.StartAcquisition(trial.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("start acquisition: %w", err)
	}

	// --- 步骤 2：多通道接收 20 个窗口 ---
	// 空化窗口区间 [8,15]，其余正常；通道 2 全程叠加机械噪声（将被标记）。
	for w := 0; w < smokeWindows; w++ {
		cavitating := w >= 8 && w <= 15
		for ch := 0; ch < smokeChannels; ch++ {
			samples := genWindow(rng, cavitating)
			if ch == 2 {
				// 通道 2 叠加机械噪声污染。
				for i := range samples {
					samples[i] += rng.NormFloat64() * 0.15
				}
			}
			startMs := int64(w) * smokeWindowMs
			if _, err := app.Segments.Ingest(trial, ch, smokeSampleRate, startMs, samples); err != nil {
				db.Close()
				return fmt.Errorf("ingest ch=%d w=%d: %w", ch, w, err)
			}
		}
	}

	// --- 步骤 3：幂等验证：重复接收同一窗口被跳过 ---
	res, err := app.Segments.Ingest(trial, 0, smokeSampleRate, 0, genWindow(rng, false))
	if err != nil {
		db.Close()
		return fmt.Errorf("re-ingest: %w", err)
	}
	if res.Inserted != 0 || res.Duplicate != 1 {
		db.Close()
		return fmt.Errorf("re-ingest should be duplicate, got inserted=%d duplicate=%d", res.Inserted, res.Duplicate)
	}

	// --- 步骤 4：通道延迟校准（通道 1 相对参考通道有相位差） ---
	if _, err := app.Segments.CalibrateChannels(trial); err != nil {
		db.Close()
		return fmt.Errorf("calibrate channels: %w", err)
	}

	// --- 步骤 5：标记通道 2 为机械噪声（不删除，仅降权） ---
	if err := markChannelNoisy(app, trial.ID, 2); err != nil {
		db.Close()
		return fmt.Errorf("mark noisy: %w", err)
	}

	// --- 步骤 6：结束采集并分析 ---
	trial, err = app.Trials.FinishAcquisition(trial.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("finish acquisition: %w", err)
	}
	noisyChannels, err := app.Segments.CountNoisyChannels(trial.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("count noisy: %w", err)
	}
	if noisyChannels != 1 {
		db.Close()
		return fmt.Errorf("expected 1 noisy channel, got %d", noisyChannels)
	}
	cfg := app.Threshold().Current()
	analyzeRes, err := app.Events.Analyze(trial, cfg, noisyChannels)
	if err != nil {
		db.Close()
		return fmt.Errorf("analyze: %w", err)
	}
	if analyzeRes.FeatureCount == 0 {
		db.Close()
		return fmt.Errorf("no features extracted")
	}
	if analyzeRes.EventCount == 0 {
		db.Close()
		return fmt.Errorf("no cavitation event detected")
	}

	events, err := app.Events.ListEvents(trial.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("list events: %w", err)
	}
	if len(events) == 0 {
		db.Close()
		return fmt.Errorf("no events persisted")
	}
	ev := events[0]
	if ev.Stage != model.EventSustained && ev.Stage != model.EventDecay && ev.Stage != model.EventInception {
		db.Close()
		return fmt.Errorf("unexpected event stage: %s", ev.Stage)
	}
	if ev.OnsetMs <= 0 {
		db.Close()
		return fmt.Errorf("event onset should be positive, got %d", ev.OnsetMs)
	}

	// --- 步骤 7：确认试验并发布结论包（封存） ---
	if _, err := app.Trials.Confirm(trial.ID); err != nil {
		db.Close()
		return fmt.Errorf("confirm trial: %w", err)
	}
	pkg, err := app.Packages.Publish(trial.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("publish package: %w", err)
	}
	if pkg.Status != model.PackagePublished {
		db.Close()
		return fmt.Errorf("package status should be published, got %s", pkg.Status)
	}

	// --- 步骤 8：封存后拒绝再接收片段 ---
	sealedTrial, err := app.Trials.Get(trial.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get sealed trial: %w", err)
	}
	if sealedTrial.Status != model.TrialSealed {
		db.Close()
		return fmt.Errorf("trial should be sealed, got %s", sealedTrial.Status)
	}
	if _, err := app.Segments.Ingest(sealedTrial, 0, smokeSampleRate, 100000, genWindow(rng, false)); err == nil {
		db.Close()
		return fmt.Errorf("sealed trial should reject segment ingest")
	}

	// --- 步骤 9：重启恢复 ---
	db.Close()

	db2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("reopen db: %w", err)
	}
	defer db2.Close()
	app2, err := service.New(db2)
	if err != nil {
		return fmt.Errorf("reinit services: %w", err)
	}
	t2, err := app2.Trials.Get(trial.ID)
	if err != nil {
		return fmt.Errorf("get trial after reopen: %w", err)
	}
	if t2.Status != model.TrialSealed {
		return fmt.Errorf("trial status after reopen should be sealed, got %s", t2.Status)
	}
	events2, err := app2.Events.ListEvents(trial.ID)
	if err != nil {
		return fmt.Errorf("list events after reopen: %w", err)
	}
	if len(events2) == 0 {
		return fmt.Errorf("no events after reopen")
	}
	pkgs2, err := app2.Packages.ListPackages(trial.ID)
	if err != nil {
		return fmt.Errorf("list packages after reopen: %w", err)
	}
	if len(pkgs2) == 0 || pkgs2[0].Status != model.PackagePublished {
		return fmt.Errorf("published package missing after reopen")
	}
	return nil
}

// markChannelNoisy 把指定通道的全部片段标记为机械噪声。
func markChannelNoisy(app *service.App, trialID string, channel int) error {
	segments, err := app.Segments.ListSegments(trialID)
	if err != nil {
		return err
	}
	for _, seg := range segments {
		if seg.ChannelIndex == channel && seg.Status != model.SegmentDuplicate {
			if err := app.Segments.MarkNoisy(seg.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
