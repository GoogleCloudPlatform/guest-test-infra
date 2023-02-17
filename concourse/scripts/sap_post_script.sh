if grep -q "ERROR - Deployment Exited" /var/log/messages; then
    echo "ERROR" > result.txt
else
    echo "SUCCESS" > result.txt
fi
gsutil cp result.txt gs://__BUCKET__/workload-tests/sap/__RUNID__/run_result
