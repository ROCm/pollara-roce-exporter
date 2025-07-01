# Pollara RoCE Exporter

A Prometheus exporter for collecting RDMA hardware metrics from Pollara 400 series NICs (AINIC) with RoCE support.
This exporter reads directly from:

```
/sys/class/infiniband/ionic_*/ports/1/hw_counters```

Instructions are provided for both **standalone** and **Docker**-based deployments.  
These examples assume an **Ubuntu** environment by default, but steps for building a CentOS-based container are also included.

---

## Metrics Overview

- All values are exposed as **counters**.  
  To visualize real-time spikes or gauge-like behavior (e.g., when a counter increases by 3 and you want to see that as a "spike" on a graph), use `irate()` in Grafana or PromQL.

### Example:

```promql
irate(res_opcode_err[1m]) * 30
```

- `1m` is the range vector interval  
- `30` is your Prometheus scrape interval in seconds  

Adjust the multiplier based on your actual scrape interval. You may need to add a few extra seconds depending on how long each scrape actually takes (sometimes longer than what you have set due to latency).

To confirm the graph is accurate:
- Create a time series graph of the metric (e.g., `res_opcode_err`)
- Add a table showing the **Delta**
- Compare the delta to your `irate()` graph to ensure the spike matches the observed metric change

---

## Listening Port

Metrics are exposed on port `9102` by default.

To change the port depends on how you deploy the exporter:
- **Docker**: Modify the `-p` flag when running the container (e.g., `-p 9200:9102`).
- **Standalone**: Edit the `http.ListenAndServe` line at the bottom of `main.go` before creating the binary.

---

## Requirements

- **Go 1.20+**

Verify your Go installation:

```bash
go version
```

### Install Go (if needed)

```bash
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
go version
```

---

## Build the Exporter (Standalone)

1. Clone this repository or create a new directory:

```bash
https://github.com/ROCm/pollara-exporter.git
cd pollara-exporter
```
If not using `git clone` due to limitations, do the following commands. Otherwise, skip this and move to step 2. 

```bash
mkdir pollara_exporter && cd pollara_exporter
go mod init pollara_exporter
vi main.go    # Paste the source from this repo
```

2. Build the binary:

```bash
go build -o pollara_exporter
```

3. If you see missing Prometheus client errors run the following or whatever it's saying it's missing:

```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

4. Run the exporter:

```bash
./pollara_exporter
```
5. Open another terminal on the same host and curl to ensure metrics are being exposed from the port:
```bash
curl localhost:9102/metrics
```
6. `pollara_exporter` is a small binary that can be moved to a permanent location. If not running in a Docker container, create a service file so that it starts at boot time. Otherwise, skip creating a service file and move onto the next section *Run with Docker*.

---

## Run with Docker

If you want to run `pollara_exporter` binary in a Docker container, follow this section of instructions.

The default `Dockerfile` in this repository is configured for **Ubuntu-based** systems. If running **CentOS-based** systems, skip to the *Using CentOS* section. 

To build and run the exporter:

```bash
docker build -t pollara-exporter .
```
There is now a pollara-exporter image in `docker images`. To create and run a container:
```bash
docker run -d \
  --name pollara-exporter \
  -p 9102:9102 \
  --restart always \
  --privileged \
  -v /sys:/sys:ro \
  pollara-exporter
```

Verify it's running correctly:

```bash
curl localhost:9102/metrics
```

## Troubleshoot Failed Container
1. Check to see if the container is running `docker ps`. If it's not there then check to see if it tried to run `docker ps -a`. 
2. Using the container ID from the output of `docker ps -a`, look at the logs `docker logs <container_ID>`. The logs will typically provide a clue of why it failed. 

### Using CentOS

If running **CentOS**, rename the provided Dockerfile before building:

```bash
mv Dockerfile_centos Dockerfile
```
Now start the steps with the first command under *Run with Docker* section above. 

---

## Example Prometheus Scrape Config
After the container is running, the prometheus.yml will need to be updated to pick up the new exporter for each host. This is an example
```yaml
- job_name: 'pollara-exporter'
  static_configs:
    - targets: ['node01:9102', 'node02:9102']
