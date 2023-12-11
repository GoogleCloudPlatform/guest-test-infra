# This simple script installs iperf on a VM and attempts to connect to an iperf
# server to test the network bandwidth between the two VMs.

outfile=$(curl http://metadata.google.internal/computeMetadata/v1/instance/hostname -H "Metadata-Flavor: Google" | cut -d"." -f1).txt
iperftarget=$(curl http://metadata.google.internal/computeMetadata/v1/instance/attributes/iperftarget -H "Metadata-Flavor: Google")
sleepduration=5
timeout=0

# Ensure the server is up and running.
echo "Checking if server is up"
timeout 2 nc -v -w 1 "$iperftarget" 5001 &> /tmp/nc_iperf
until [[ $(< /tmp/nc_iperf) == *"succeeded"* || $(< /tmp/nc_iperf) == *"Connected"* || "$timeout" -ge "$maxtimeout" ]]; do
  cat /tmp/nc_iperf
  echo Failed to connect to server. Trying again in 5s
  sleep "$sleepduration"
  timeout=$((timeout+sleepduration))

  # timeout ensures the command stops. On some versions of netcat,
  # the -w flag seems nonfunctional. This is the workaround.
  timeout 2 nc -v -w 1 "$iperftarget" 5001 &> /tmp/nc_iperf
done
if [[ $timeout -ge $maxtimeout ]]; then
  exit 1
fi
sleep "$sleepduration"

# Run iperf
echo "$(date +"%Y-%m-%d %T"): Running iperf client with target $iperftarget"
iperf -t 30 -c "$iperftarget" -P $parallelcount | tee /tmp/iperf_results
results=$(cat /tmp/iperf_results | grep SUM | tr -s ' ' 2>&1 | tee -a "$outfile")

echo "$(date +"%Y-%m-%d %T"): Test Results $results"
echo "$(date +"%Y-%m-%d %T"): Sending results to metadata."
for i in $(seq 0 2); do
sleep $i
curl -X PUT --data "$results" http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/results -H "Metadata-Flavor: Google" && break
done
