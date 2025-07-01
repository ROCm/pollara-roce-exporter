package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Ironic NIC-specific metric definitions (counters and gauges)
var metricDefinitions = []struct {
	name string
	help string
	kind string // gauge or counter
}{
	// General NIC lifespan not sure what this actually means
	{"lifespan", "NIC lifespan in seconds", "gauge"},

	// Request RX Errors
	{"req_rx_cqe_err", "Request RX CQE Errors", "counter"},
	{"req_rx_cqe_flush", "Request RX CQE Flushes", "counter"},
	{"req_rx_dup_response", "Duplicate RX responses", "counter"},
	{"req_rx_impl_nak_seq_err", "Request RX NAK sequence errors", "counter"},
	{"req_rx_inval_pkts", "Invalid RX packets", "counter"},
	{"req_rx_oper_err", "Request RX operation errors", "counter"},
	{"req_rx_pkt_seq_err", "Packet sequence errors", "counter"},
	{"req_rx_rmt_acc_err", "Remote access errors", "counter"},
	{"req_rx_rmt_req_err", "Remote request errors", "counter"},
	{"req_rx_rnr_retry_err", "RNR retry errors", "counter"},

	// Request TX Errors
	{"req_tx_loc_acc_err", "Local TX access errors", "counter"},
	{"req_tx_loc_oper_err", "Local TX operation errors", "counter"},
	{"req_tx_loc_sgl_inv_err", "Local TX scatter-gather list invalid errors", "counter"},
	{"req_tx_mem_mgmt_err", "TX memory management errors", "counter"},
	{"req_tx_retry_excd_err", "TX retries exceeded", "counter"},

	// Response RX Errors
	{"resp_rx_cqe_err", "Response RX CQE Errors", "counter"},
	{"resp_rx_cqe_flush", "Response RX CQE Flushes", "counter"},
	{"resp_rx_dup_request", "Duplicate RX requests", "counter"},
	{"resp_rx_inval_request", "Invalid RX requests", "counter"},
	{"resp_rx_loc_len_err", "Local length errors", "counter"},
	{"resp_rx_loc_oper_err", "Local operation errors", "counter"},
	{"resp_rx_outof_atomic", "Out of atomic resources", "counter"},
	{"resp_rx_outof_buf", "Out of buffer space", "counter"},
	{"resp_rx_outouf_seq", "Out of sequence errors", "counter"},
	{"resp_rx_s0_table_err", "Table errors in RX response", "counter"},

	// Response TX Errors
	{"resp_tx_loc_sgl_inv_err", "Local TX scatter-gather list invalid errors", "counter"},
	{"resp_tx_pkt_seq_err", "Response TX Packet Sequence Errors", "counter"},
	{"resp_tx_rmt_acc_err", "Remote TX access errors", "counter"},
	{"resp_tx_rmt_inval_req_err", "Remote TX invalid request errors", "counter"},
	{"resp_tx_rmt_oper_err", "Remote TX operation errors", "counter"},
	{"resp_tx_rnr_retry_err", "Remote TX RNR retry errors", "counter"},

	// RX (Received) RDMA Metrics
	{"rx_rdma_cnp_pkts", "Received RDMA Congestion Notification Packets", "counter"},
	{"rx_rdma_ecn_pkts", "Received RDMA ECN Marked Packets", "counter"},
	{"rx_rdma_mcast_bytes", "Received RDMA Multicast Bytes", "counter"},
	{"rx_rdma_mcast_pkts", "Received RDMA Multicast Packets", "counter"},
	{"rx_rdma_ucast_bytes", "Received RDMA Unicast Bytes", "counter"},
	{"rx_rdma_ucast_pkts", "Received RDMA Unicast Packets", "counter"},

	// TX (Transmitted) RDMA Metrics
	{"tx_rdma_cnp_pkts", "Transmitted RDMA Congestion Notification Packets", "counter"},
	{"tx_rdma_mcast_bytes", "Transmitted RDMA Multicast Bytes", "counter"},
	{"tx_rdma_mcast_pkts", "Transmitted RDMA Multicast Packets", "counter"},
	{"tx_rdma_ucast_bytes", "Transmitted RDMA Unicast Bytes", "counter"},
	{"tx_rdma_ucast_pkts", "Transmitted RDMA Unicast Packets", "counter"},
}

var (
	customRegistry = prometheus.NewRegistry()
	metrics        = make(map[string]*prometheus.GaugeVec)
	counters       = make(map[string]*prometheus.CounterVec)
)

// Register Prometheus metrics dynamically
func registerMetrics() {
	for _, def := range metricDefinitions {
		switch def.kind {
		case "gauge":
			metrics[def.name] = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: def.name,
					Help: def.help,
				},
				[]string{"nic"},
			)
			customRegistry.MustRegister(metrics[def.name])
		case "counter":
			counters[def.name] = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: def.name,
					Help: def.help,
				},
				[]string{"nic"},
			)
			customRegistry.MustRegister(counters[def.name])
		}
	}
}

// Discover only Ironic NICs (ionic_*)
func discoverNICs() ([]string, error) {
	nicPaths, err := filepath.Glob("/sys/class/infiniband/ionic_*")
	if err != nil {
		return nil, fmt.Errorf("failed to list NIC paths: %w", err)
	}

	if len(nicPaths) == 0 {
		return nil, fmt.Errorf("no Ironic NICs found in /sys/class/infiniband/")
	}

	fmt.Printf("Discovered Ironic NICs: %v\n", nicPaths)
	return nicPaths, nil
}

// Parse and update metrics for each NIC
func parseAndUpdateMetrics(nicPath string) error {
	nicName := filepath.Base(nicPath)
	hwCountersPath := filepath.Join(nicPath, "ports/1/hw_counters")

	if _, err := os.Stat(hwCountersPath); os.IsNotExist(err) {
		fmt.Printf("NIC %s does not have hw_counters, skipping...\n", nicName)
		return nil
	}

	for _, def := range metricDefinitions {
		counterFile := filepath.Join(hwCountersPath, def.name)
		data, err := ioutil.ReadFile(counterFile)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Metric file %s missing for NIC %s, skipping...\n", def.name, nicName)
				continue
			}
			return fmt.Errorf("error reading %s: %w", counterFile, err)
		}

		valueStr := strings.TrimSpace(string(data))
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return fmt.Errorf("error parsing value from %s: %w", counterFile, err)
		}

		switch def.kind {
		case "gauge":
			if gauge, exists := metrics[def.name]; exists {
				gauge.WithLabelValues(nicName).Set(value)
			}
		case "counter":
			if counter, exists := counters[def.name]; exists {
				counter.WithLabelValues(nicName).Add(value)
			}
		}
	}
	return nil
}

// Update metrics for all discovered NICs
func updateMetrics(nicPaths []string) {
	for _, nicPath := range nicPaths {
		err := parseAndUpdateMetrics(nicPath)
		if err != nil {
			fmt.Printf("Error updating metrics for NIC %s: %v\n", nicPath, err)
		}
	}
}

func main() {
	registerMetrics()
	nicPaths, err := discoverNICs()
	if err != nil {
		fmt.Printf("Error discovering NICs: %v\n", err)
		os.Exit(1)
	}

	go func() {
		for {
			updateMetrics(nicPaths)
			time.Sleep(15 * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(customRegistry, promhttp.HandlerOpts{}))
	fmt.Println("Serving Ironic NIC metrics on :9102")
	http.ListenAndServe(":9102", nil)
}
