if cat /var/log/messages | grep -q "ERROR - Deployment Exited"; then
    echo "ERROR" > result.txt
else
    echo "SUCCESS" > result.txt
fi
gsutil cp result.txt gs://$1/workload-tests/sap/$2/run_result
