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

// TestRandomReadIOPS checks that random read IOPS are around the value listed in public docs.
func TestRandomReadIOPS(t *testing.T) {
	var randReadIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if randReadIOPSJson, err = runFIOWindows(t, randRead); err != nil {
			t.Fatalf("windows fio rand read failed with error: %v. If testing locally, check the guidance at storageperf/startupscripts/install_fio.ps1", err)
		}
	} else {
		if randReadIOPSJson, err = runFIOLinux(t, randRead); err != nil {
			t.Fatalf("linux fio rand read failed with error: %s", err.Error())
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(randReadIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(randReadIOPSJson), err)
	}

	// this is a json.Number object
	finalIOPSValueNumber := fioOut.Jobs[0].ReadResult.IOPS
	var finalIOPSValue float64
	if finalIOPSValue, err = finalIOPSValueNumber.Float64(); err != nil {
		t.Fatalf("iops json number %s was not a float: %v", finalIOPSValueNumber.String(), err)
	}
	finalIOPSValueString := fmt.Sprintf("%f", finalIOPSValue)
	expectedRandReadIOPSString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", randReadAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribute %s: err %v", randReadAttribute, err)
	}

	expectedRandReadIOPSString = strings.TrimSpace(expectedRandReadIOPSString)
	var expectedRandReadIOPS float64
	if expectedRandReadIOPS, err = strconv.ParseFloat(expectedRandReadIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %s was not a float: err %v", expectedRandReadIOPSString, err)
	}

	machineName, _ := utils.GetInstanceName(utils.Context(t))
	if finalIOPSValue < iopsErrorMargin*expectedRandReadIOPS {
		t.Fatalf("iops average for vm %s was too low: expected at least %f of target %s, got %s", machineName, iopsErrorMargin, expectedRandReadIOPSString, finalIOPSValueString)
	}

	t.Logf("iops test pass for vm %s with %s iops, expected at least %f of target %s", machineName, finalIOPSValueString, iopsErrorMargin, expectedRandReadIOPSString)
}

// TestSequentialReadIOPS checks that sequential read IOPS are around the value listed in public docs.
func TestSequentialReadIOPS(t *testing.T) {
	var seqReadIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if seqReadIOPSJson, err = runFIOWindows(t, seqRead); err != nil {
			t.Fatalf("windows fio seq read failed with error: %v", err)
		}
	} else {
		if seqReadIOPSJson, err = runFIOLinux(t, seqRead); err != nil {
			t.Fatalf("linux fio seq read failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(seqReadIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(seqReadIOPSJson), err)
	}

	// bytes is listed in bytes per second in the fio output
	var finalBandwidthBytesPerSecond int64 = 0
	for _, job := range fioOut.Jobs {
		// this is the bandwidth units/sec as a json.Number object
		bandwidthNumber := job.ReadResult.Bandwidth
		var bandwidthInt int64
		if bandwidthInt, err = bandwidthNumber.Int64(); err != nil {
			t.Fatalf("bandwidth units per second %s was not a float: err  %v", bandwidthNumber.String(), err)
		}
		finalBandwidthBytesPerSecond += bandwidthInt * fioBWToBytes
	}

	var finalBandwidthMBps float64 = float64(finalBandwidthBytesPerSecond) / bytesInMB
	finalBandwidthMBpsString := fmt.Sprintf("%f", finalBandwidthMBps)

	expectedSeqReadIOPSString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", seqReadAttribute)
	if err != nil {
		t.Fatalf("could not get guest metadata %s: err r%v", seqReadAttribute, err)
	}

	expectedSeqReadIOPSString = strings.TrimSpace(expectedSeqReadIOPSString)
	var expectedSeqReadIOPS float64
	if expectedSeqReadIOPS, err = strconv.ParseFloat(expectedSeqReadIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %s  was not a float: err %v", expectedSeqReadIOPSString, err)
	}

	// suppress the error because the vm name is only for printing out test results, and does not affect test behavior
	machineName, _ := utils.GetInstanceName(utils.Context(t))
	if finalBandwidthMBps < iopsErrorMargin*expectedSeqReadIOPS {
		t.Fatalf("iops average was too low for vm %s: expected at least %f of target %s, got %s", machineName, iopsErrorMargin, expectedSeqReadIOPSString, finalBandwidthMBpsString)
	}

	t.Logf("iops test pass for vm %s with %s iops, expected at least %f of target %s", machineName, finalBandwidthMBpsString, iopsErrorMargin, expectedSeqReadIOPSString)
}
