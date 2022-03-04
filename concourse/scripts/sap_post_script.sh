if cat /var/log/messages | grep -q "ERROR - Deployment Exited"; then
    echo "ERROR" > result.txt
else
    echo "SUCCESS" > result.txt
fi
gsutil cp result.txt gs://__BUCKET__/workload-tests/sap/__RUNID__/run_result
