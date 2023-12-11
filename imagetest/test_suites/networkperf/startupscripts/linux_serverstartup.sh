# This script installs iperf on a VM and starts an iperf server for the client
# to test the network bandwidth between the two VMs.

echo "Starting iperf server"
timeout $maxtimeout iperf -s -P 1
timeout $maxtimeout iperf -s -P $parallelcount
