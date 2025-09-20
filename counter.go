package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	charts "github.com/vicanso/go-charts/v2"
)

type metric struct {
	mutex      sync.Mutex
	datapoints map[string]datapoint
}

type metricList struct {
	Metrics []string `json:"metrics"`
}

type datapoint struct {
	Value   float64 `json:"value"`
	Count   int     `json:"count"`
	Average float64 `json:"average"`
}

type error struct {
	Error string `json:"error"`
}

type status struct {
	Status string `json:"status"`
}

var data map[string]*metric
var globalLock sync.Mutex

//go:embed Readme.md
var readme []byte

const persistenceInterval = 30 * time.Second
const lruInterval = 5 * time.Minute
const retentionDuration = time.Duration(6 * time.Hour)
const dataDir = "data"

func getIndex(w http.ResponseWriter, r *http.Request) {
	w.Write(readme)
}

func getMetricList(w http.ResponseWriter, r *http.Request) {
	globalLock.Lock()
	var metrics []string
	for k := range data {
		metrics = append(metrics, k)
	}
	sort.Strings(metrics)
	globalLock.Unlock()
	w.Header().Set("Content-Type", "application/json")
	response, err := json.Marshal(metricList{Metrics: metrics})
	if err != nil {
		panic(err)
	}
	w.Write(response)
}

func getMetric(w http.ResponseWriter, r *http.Request) {
	metricName := chi.URLParam(r, "metricName")
	w.Header().Set("Content-Type", "application/json")

	// Find the requested data
	globalLock.Lock()
	m, ok := data[metricName]
	globalLock.Unlock()
	if ok {
		m.mutex.Lock()
		response, err := json.Marshal(m.datapoints)
		if err != nil {
			panic(err)
		}
		m.mutex.Unlock()
		w.Write(response)
	} else {
		response, err := json.Marshal(error{Error: "Could not find metric"})
		if err != nil {
			panic(err)
		}
		w.Write(response)
	}
}

func getMetricChart(w http.ResponseWriter, r *http.Request) {
	metricName := chi.URLParam(r, "metricName")
	dimensionName := chi.URLParam(r, "dimensionName")

	// Find the requested data
	globalLock.Lock()
	m, ok := data[metricName]
	globalLock.Unlock()
	if ok {
		m.mutex.Lock()

		// Get list of keys in order
		var keys []string
		for k := range m.datapoints {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Build up dataset
		XValues := make([]string, 0)
		YValues := make([]float64, 0)
		for _, k := range keys {
			v := m.datapoints[k]

			// Reformat date
			t, err := time.Parse("2006-01-02 15:04", k)
			if err != nil {
				panic(err)
			}
			XValues = append(XValues, t.Format("03:04 PM"))

			switch dimensionName {
			case "sum":
				YValues = append(YValues, v.Value)
			case "count":
				YValues = append(YValues, float64(v.Count))
			default:
				YValues = append(YValues, v.Average)
			}
		}

		// Generate chart
		p, err := charts.LineRender(
			[][]float64{YValues},
			charts.TitleOptionFunc(charts.TitleOption{
				Text: metricName,
				Left: "50%",
			}),
			charts.XAxisOptionFunc(charts.XAxisOption{
				Data:        XValues,
				BoundaryGap: charts.FalseFlag(),
				FontSize:    10,
			}),
			charts.YAxisOptionFunc(charts.YAxisOption{
				FontSize: 10,
			}),
			charts.HeightOptionFunc(400),
			charts.WidthOptionFunc(1024),
			func(opt *charts.ChartOption) {
				opt.SymbolShow = charts.FalseFlag()
			},
		)

		if err != nil {
			panic(err)
		}

		buf, err := p.Bytes()
		if err != nil {
			panic(err)
		}

		m.mutex.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Write(buf)
	} else {
		w.Header().Set("Content-Type", "application/json")
		response, err := json.Marshal(error{Error: "Could not find metric"})
		if err != nil {
			panic(err)
		}
		w.Write(response)
	}
}

func writeMetrics(writeBuffer chan map[string]datapoint) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var sample map[string]datapoint
		err := decoder.Decode(&sample)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			response, _ := json.Marshal(error{Error: "Could not parse incoming metrics"})
			w.WriteHeader(http.StatusBadRequest)
			w.Write(response)
			return
		}

		// Queue data for processing and return response
		writeBuffer <- sample
		w.Header().Set("Content-Type", "application/json")
		response, _ := json.Marshal(status{Status: "Queued"})
		w.Write(response)
	}
}

func appendMetrics(writeBuffer chan map[string]datapoint) {
	for {
		sample := <-writeBuffer
		timeKey := time.Now().Format("2006-01-02 15:04")
		for metricName, d := range sample {
			globalLock.Lock()
			m, ok := data[metricName]
			globalLock.Unlock()

			// Write count as 1 if no value received
			if d.Count == 0 {
				d.Count = 1
			}

			// Increment point if it exists, otherwise create a new point
			if ok {
				m.mutex.Lock()
				if point, ok := m.datapoints[timeKey]; ok {
					point.Count += d.Count
					point.Value += d.Value
					point.Average = point.Value / float64(point.Count)
					m.datapoints[timeKey] = point
				} else {
					point := datapoint{Count: d.Count, Value: d.Value, Average: d.Value / float64(d.Count)}
					m.datapoints[timeKey] = point
				}
				m.mutex.Unlock()
			} else {
				globalLock.Lock()
				data[metricName] = &metric{
					datapoints: make(map[string]datapoint, 0),
				}
				globalLock.Unlock()
			}
		}
	}
}

func restore() {
	globalLock.Lock()
	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		fmt.Println("Data directory could not be read, restore skipped")
		return
	}
	for _, f := range files {
		metricName := strings.Replace(f.Name(), ".json", "", 1)
		datapoints := make(map[string]datapoint, 0)
		file, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", dataDir, f.Name()))
		if err != nil {
			fmt.Printf("Could not read %s/%s\n", dataDir, f.Name())
			continue
		}
		json.Unmarshal(file, &datapoints)
		data[metricName] = &metric{
			datapoints: datapoints,
		}
	}
	globalLock.Unlock()
}

func lru() {
	for {
		// Determine cut-off point
		oldestTimeKey := time.Now().Add(-1 * retentionDuration).Format("2006-01-02 15:04")

		// Get unique metric names
		var metricNames []string
		globalLock.Lock()
		for metricName := range data {
			metricNames = append(metricNames, metricName)
		}
		globalLock.Unlock()

		for _, metricName := range metricNames {
			// Get metric
			globalLock.Lock()
			m, ok := data[metricName]
			if !ok {
				continue
			}
			globalLock.Unlock()

			// Remove old data
			m.mutex.Lock()
			for timeKey := range m.datapoints {
				if timeKey < oldestTimeKey {
					delete(m.datapoints, timeKey)
					if debugMode() {
						fmt.Printf("Removing old datapoint %s for %s\n", timeKey, metricName)
					}
				}
			}
			m.mutex.Unlock()
		}
		time.Sleep(lruInterval)
	}
}

func persist() {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		os.Mkdir(dataDir, 0700)
	}
	for {
		time.Sleep(persistenceInterval)
		globalLock.Lock()
		for metricName, m := range data {
			m.mutex.Lock()
			file, err := os.OpenFile(
				fmt.Sprintf("%s/%s.json", dataDir, metricName),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			if err != nil {
				fmt.Printf("Could not write %s to disk\n", metricName)
				continue
			}
			encoder := json.NewEncoder(file)
			encoder.Encode(m.datapoints)
			file.Close()
			if debugMode() {
				fmt.Printf("Persisted %s to disk\n", metricName)
			}
			m.mutex.Unlock()
		}
		globalLock.Unlock()
	}
}

func debugMode() bool {
	return os.Getenv("DEBUG") != ""
}

func main() {
	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	writeBuffer := make(chan map[string]datapoint, 1000)
	r.Get("/", getIndex)
	r.Get("/metric", getMetricList)
	r.Post("/metric", writeMetrics(writeBuffer))
	r.Get("/metric/{metricName:[a-zA-Z0-9-.]+}", getMetric)
	r.Get("/metric/{metricName:[a-zA-Z0-9-.]+}/{dimensionName:[a-z]+}.png", getMetricChart)

	// Parse command line options
	port := flag.Int("port", 8080, "port on which to listen")
	flag.Parse()

	// Set up global data store
	data = make(map[string]*metric)
	restore()

	// Set up buffered writer for incoming data
	go appendMetrics(writeBuffer)

	// LRU goroutine
	go lru()

	// Peristence goroutine
	go persist()

	// Start server
	fmt.Printf("Counter server started on port %d\n", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), r); err != nil {
		panic(err)
	}
}
