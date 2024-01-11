//go:build cit
// +build cit

package storageperf

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// TestRandomWriteIOPS checks that random write IOPS are around the value listed in public docs.
func TestRandomWriteIOPS(t *testing.T) {
	var randWriteIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if randWriteIOPSJson, err = runFIOWindows(randWrite); err != nil {
			t.Fatalf("windows fio rand write failed with error: %v", err)
		}
	} else {
		if randWriteIOPSJson, err = runFIOLinux(t, randWrite); err != nil {
			t.Fatalf("linux fio rand write failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(randWriteIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(randWriteIOPSJson), err)
	}

	// this is a json.Number object
	finalIOPSValueNumber := fioOut.Jobs[0].WriteResult.IOPS
	var finalIOPSValue float64
	if finalIOPSValue, err = finalIOPSValueNumber.Float64(); err != nil {
		t.Fatalf("iops string %s was not a float: err %v", finalIOPSValueNumber.String(), err)
	}
	finalIOPSValueString := fmt.Sprintf("%f", finalIOPSValue)
	expectedRandWriteIOPSString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", randWriteAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribut %s: err %v", randWriteAttribute, err)
	}

	expectedRandWriteIOPSString = strings.TrimSpace(expectedRandWriteIOPSString)
	var expectedRandWriteIOPS float64
	if expectedRandWriteIOPS, err = strconv.ParseFloat(expectedRandWriteIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %s was not a float: err %v", expectedRandWriteIOPSString, err)
	}

	// suppress the error because the vm name is only for printing out test results, and does not affect test behavior
	machineName, _ := utils.GetInstanceName(utils.Context(t))
	if finalIOPSValue < iopsErrorMargin*expectedRandWriteIOPS {
		t.Fatalf("iops average for vm %s was too low: expected at least %f of target %s, got %s", machineName, iopsErrorMargin, expectedRandWriteIOPSString, finalIOPSValueString)
	}

	t.Logf("iops test pass for vm %s with %s iops, expected at least %f of target %s", machineName, finalIOPSValueString, iopsErrorMargin, expectedRandWriteIOPSString)
}

// TestSequentialWriteIOPS checks that sequential write IOPS are around the value listed in public docs.
func TestSequentialWriteIOPS(t *testing.T) {
	var seqWriteIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if seqWriteIOPSJson, err = runFIOWindows(seqWrite); err != nil {
			t.Fatalf("windows fio seq write failed with error: %v", err)
		}
	} else {
		if seqWriteIOPSJson, err = runFIOLinux(t, seqWrite); err != nil {
			t.Fatalf("linux fio seq write failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(seqWriteIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(seqWriteIOPSJson), err)
	}

	var finalBandwidthBytesPerSecond int64 = 0
	for _, job := range fioOut.Jobs {
		// this is a json.Number object
		bandwidthNumber := job.WriteResult.Bandwidth
		var bandwidthInt int64
		if bandwidthInt, err = bandwidthNumber.Int64(); err != nil {
			t.Fatalf("bandwidth units per sec %s was not an int: err %v", bandwidthNumber.String(), err)
		}
		finalBandwidthBytesPerSecond += bandwidthInt * fioBWToBytes
	}
	var finalBandwidthMBps float64 = float64(finalBandwidthBytesPerSecond) / bytesInMB
	finalBandwidthMBpsString := fmt.Sprintf("%f", finalBandwidthMBps)

	expectedSeqWriteIOPSString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", seqWriteAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribute %s: err %v", seqWriteAttribute, err)
	}

	expectedSeqWriteIOPSString = strings.TrimSpace(expectedSeqWriteIOPSString)
	var expectedSeqWriteIOPS float64
	if expectedSeqWriteIOPS, err = strconv.ParseFloat(expectedSeqWriteIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %s was not a float: err %v", expectedSeqWriteIOPSString, err)
	}

	machineName, _ := utils.GetInstanceName(utils.Context(t))
	if finalBandwidthMBps < iopsErrorMargin*expectedSeqWriteIOPS {
		t.Fatalf("iops average for vm %s was too low: expected at least %f of target %s, got %s", machineName, iopsErrorMargin, expectedSeqWriteIOPSString, finalBandwidthMBpsString)
	}

	t.Logf("iops test pass for vm %s with %s iops, expected at least %f of target %s", machineName, finalBandwidthMBpsString, iopsErrorMargin, expectedSeqWriteIOPSString)
}
