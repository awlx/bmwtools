package data

import (
	"errors"
	"testing"
	"time"
)

func TestEpochRecordsAreSkipped(t *testing.T) {
	m := NewManager()
	raws := []RawSession{{}, {StartTime: 1735718400, EndTime: 1735722000, DisplayedStartSoc: 20, DisplayedSoc: 80}}
	if err := m.ProcessRawSessions(raws); err != nil {
		t.Fatal(err)
	}
	if n := len(m.GetSessions()); n != 1 {
		t.Fatalf("sessions = %d, want 1", n)
	}
}

func TestNoValidSessionsIsAnError(t *testing.T) {
	m := NewManager()
	if err := m.ProcessRawSessions([]RawSession{{}, {}}); !errors.Is(err, ErrNoValidSessions) {
		t.Fatalf("err = %v, want ErrNoValidSessions", err)
	}
	if err := m.ProcessRawSessions(nil); !errors.Is(err, ErrNoValidSessions) {
		t.Fatalf("empty: err = %v", err)
	}
}

func TestSessionEndTime(t *testing.T) {
	const start = int64(1735718400)
	normal := RawSession{StartTime: start, EndTime: start + 3600}
	if got := sessionEndTime(normal); got.Unix() != start+3600 {
		t.Errorf("normal end = %v", got)
	}

	blocks := RawSession{StartTime: start, TotalChargingDurationSec: 600}
	blocks.ChargingBlocks = append(blocks.ChargingBlocks,
		struct {
			AveragePowerGridKw float64 `json:"averagePowerGridKw"`
			StartTime          int64   `json:"startTime"`
			EndTime            int64   `json:"endTime"`
		}{StartTime: start, EndTime: start + 1800},
		struct {
			AveragePowerGridKw float64 `json:"averagePowerGridKw"`
			StartTime          int64   `json:"startTime"`
			EndTime            int64   `json:"endTime"`
		}{StartTime: start + 1800, EndTime: start + 2700})
	if got := sessionEndTime(blocks); got.Unix() != start+2700 {
		t.Errorf("end from blocks = %v", got)
	}

	duration := RawSession{StartTime: start, TotalChargingDurationSec: 900}
	if got := sessionEndTime(duration); got.Unix() != start+900 {
		t.Errorf("end from duration = %v", got)
	}

	if got := sessionEndTime(RawSession{StartTime: start}); !got.Equal(time.Time{}) {
		t.Errorf("unknown end = %v, want zero time", got)
	}
}
