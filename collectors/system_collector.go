// go:build ignore
// Sample usage:
//   go mod init example.com/sysmetrics && go get github.com/shirou/gopsutil/v3@latest
//   go run main.go -report=10 -host=localhost -port=8080
//
// Posts payload like:
// {
//   "system.cpu.percent":            {"count":10,"value":654.3},   // avg = 65.43
//   "system.mem.used_percent":       {"count":10,"value":420.0},   // avg = 42.0
//   "system.cpu.temp_c":             {"count":10,"value":645.0},   // if available
//   "system.net.recv_bytes_per_sec": {"count":10,"value":1234567},
//   "system.net.sent_bytes_per_sec": {"count":10,"value":2345678}
// }

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type Metric struct {
	Count int     `json:"count"`
	Value float64 `json:"value"` // sum of samples over the window
}

type agg map[string]*Metric

func (a agg) add(name string, v float64) {
	if m, ok := a[name]; ok {
		m.Count++
		m.Value += v
	} else {
		a[name] = &Metric{Count: 1, Value: v}
	}
}

func main() {
	reportEvery := flag.Int("report", 5, "reporting interval in seconds")
	apiHost := flag.String("host", "localhost", "API hostname")
	apiPort := flag.Int("port", 8080, "API port")
	flag.Parse()

	client := &http.Client{Timeout: 5 * time.Second}
	endpoint := fmt.Sprintf("http://%s:%d/metric", *apiHost, *apiPort)

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	var (
		a           = make(agg)
		lastNetRecv uint64
		lastNetSent uint64
		primedNet   bool
		lastReport  = time.Now()
	)

	for {
		select {
		case <-stop:
			return
		default:
		}

		// ---- 1) SAMPLE ONCE PER SECOND ----
		// CPU% measured over ~1 second
		if pct, err := cpu.Percent(time.Second, false); err == nil && len(pct) > 0 {
			a.add("system.cpu.percent", pct[0])
		} else if err != nil {
			log.Printf("cpu.Percent error: %v", err)
		}

		// Mem%
		if vm, err := mem.VirtualMemory(); err == nil {
			a.add("system.mem.used_percent", vm.UsedPercent)
		} else {
			log.Printf("mem.VirtualMemory error: %v", err)
		}

		// CPU temp (best-effort; may be unavailable)
		if temp, ok := cpuTempC(); ok {
			a.add("system.cpu.temp_c", temp)
		}

		// NIC rx/tx bytes/sec (sum over non-loopback)
		if recv, sent, ok := netTotals(); ok {
			if primedNet {
				rx := float64(recv - lastNetRecv) // since last second
				tx := float64(sent - lastNetSent)
				if rx < 0 {
					rx = 0
				}
				if tx < 0 {
					tx = 0
				}
				a.add("system.net.recv_bytes_per_sec", rx)
				a.add("system.net.sent_bytes_per_sec", tx)
			}
			lastNetRecv, lastNetSent = recv, sent
			primedNet = true
		}

		// ---- 2) REPORT EVERY N SECONDS ----
		if time.Since(lastReport) >= time.Duration(*reportEvery)*time.Second {
			if len(a) > 0 {
				if err := postJSON(client, endpoint, a); err != nil {
					log.Printf("POST %s failed: %v", endpoint, err)
				}
			}
			a = make(agg) // reset
			lastReport = time.Now()
		}
	}
}

func postJSON(client *http.Client, url string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	log.Writer().Write(b)
	log.Writer().Write([]byte("\n\n"))
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}

func netTotals() (recv, sent uint64, ok bool) {
	stats, err := net.IOCounters(true)
	if err != nil {
		return 0, 0, false
	}
	for _, s := range stats {
		name := strings.ToLower(s.Name)
		if name == "lo" || strings.HasPrefix(name, "lo") {
			continue
		}
		recv += s.BytesRecv
		sent += s.BytesSent
	}
	return recv, sent, true
}

func cpuTempC() (float64, bool) {
	temps, err := host.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return 0, false
	}
	var sum float64
	var n int
	for _, t := range temps {
		key := strings.ToLower(t.SensorKey)
		if strings.Contains(key, "cpu") || strings.Contains(key, "package") || strings.Contains(key, "coretemp") || strings.Contains(key, "tdie") || strings.Contains(key, "tctl") || strings.Contains(key, "cpu_thermal") {
			sum += t.Temperature
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}
