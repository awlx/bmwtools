package data

import "testing"

func TestCleanProviderNameRealNames(t *testing.T) {
	cases := map[string]string{
		"HPC IONITY":                               "IONITY",
		"HPC/DC IONITY":                            "IONITY",
		"HPC/DC IONITY GmbH":                       "IONITY",
		"HPC/DC/AC IONITY GmbH":                    "IONITY",
		"HPC/DC/AC EnBW mobility+":                 "EnBW",
		"HPC/AC EnBW mobility+":                    "EnBW",
		"HPC EnBW mobility+":                       "EnBW",
		"DC EnBW Energie Baden-Württemberg AG":     "EnBW",
		"HPC Tesla Supercharger":                   "Tesla",
		"HPC/DC Tesla Supercharger":                "Tesla",
		"HPC Aral pulse":                           "Aral pulse",
		"HPC EWE Go GmbH":                          "EWE Go",
		"AC Vaylens GmbH":                          "Vaylens",
		"AC reev":                                  "reev",
		"DC/AC Kaufland Stiftung GmbH und Co. KG":  "Kaufland",
		"DC/AC Lidl Dienstleistung GmbH & Co. KG":  "Lidl",
		"AC Albwerk GmbH & Co. KG":                 "Albwerk",
		"HPC SMATRICS GmbH & Co KG":                "SMATRICS",
		"AC Stadtwerke München GmbH":               "Stadtwerke München",
		"AC Stadtwerke Nürtingen GmbH":             "Stadtwerke Nürtingen",
		"HPC Mer Germany GmbH":                     "Mer",
		"HPC Mer Austria":                          "Mer",
		"HPC/DC E.ON Drive Infrastructure GmbH":    "E.ON",
		"AC ChargePoint Austria GmbH":              "ChargePoint",
		"AC Chargepoint Germany":                   "ChargePoint",
		"HPC/DC/AC TotalEnergies HPC":              "TotalEnergies",
		"AC TOTAL Deutschland GmbH":                "TotalEnergies",
		"HPC/DC Fastned Deutschland GmbH & Co. KG": "Fastned",
		"AC Allego B.V.":                           "Allego",
		"DC/AC Power Dot France SASU":              "Powerdot",
		"HPC/DC Grupo EasyCharger S.A.":            "Grupo EasyCharger",
		"AC Enel X Way Italia S.r.l.":              "Enel",
		"DC/AC HEP - Opskrba d.o.o.":               "HEP - Opskrba",
		"HPC/DC/AC Monta ApS":                      "Monta",
		"HPC Vattenfall InCharge AB":               "Vattenfall",
		"  HPC   Allego  ":                         "Allego",
		"DCS":                                      "DCS",
		"ACME Charging":                            "ACME Charging",
		"Energie AG Oberösterreich":                "Energie AG Oberösterreich",
		"HPC/DC Groupe E / MOVE Mobility AG":       "MOVE Mobility",
	}
	for raw, want := range cases {
		if got := CleanProviderName(raw); got != want {
			t.Errorf("CleanProviderName(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestCleanProviderNameRejectsNonProviders(t *testing.T) {
	for _, raw := range []string{
		"", " ", "Unknown", "unknown", "HPC", "HPC/DC", "AC Charging operator",
		"123", "--", "A", "AC Aucun / None", "Hauptstraße 12", "Musterweg 3, 80331 München",
	} {
		if got := CleanProviderName(raw); got != "" {
			t.Errorf("CleanProviderName(%q) = %q, want empty", raw, got)
		}
	}
}

func TestCleanProviderNameIsIdempotent(t *testing.T) {
	for _, raw := range []string{"HPC/DC IONITY GmbH", "AC Stadtwerke München GmbH", "AC reev", "DC/AC Kaufland Stiftung GmbH und Co. KG"} {
		once := CleanProviderName(raw)
		if twice := CleanProviderName(once); twice != once {
			t.Errorf("not idempotent: %q -> %q -> %q", raw, once, twice)
		}
	}
}

func TestVoteProviderMajority(t *testing.T) {
	v := voteProvider([]string{"HPC/AC Pfalzwerke AG", "HPC IONITY", "HPC/DC IONITY GmbH", "HPC/DC/AC IONITY"})
	if v.winner != "IONITY" || len(v.tied) != 0 {
		t.Fatalf("got %+v, want IONITY without tie", v)
	}
	v = voteProvider([]string{"HPC Allego", "Unknown", ""})
	if v.winner != "Allego" {
		t.Fatalf("got %+v, want Allego", v)
	}
	if v = voteProvider([]string{"Unknown", ""}); v.winner != "" {
		t.Fatalf("got %+v, want no winner", v)
	}
	v = voteProvider([]string{"HPC Allego", "AC Fastned", "HPC/DC Allego", "DC Fastned"})
	if v.winner != "Allego" || len(v.tied) != 2 {
		t.Fatalf("got %+v, want tie Allego/Fastned with first match first", v)
	}
}

func rawWithMatches(start int64, lat, lon float64, names ...string) RawSession {
	r := RawSession{StartTime: start, EndTime: start + 3600, DisplayedStartSoc: 20, DisplayedSoc: 80, EnergyConsumedFromPowerGridKwh: 50}
	r.ChargingLocation.MapMatchedLatitude = lat
	r.ChargingLocation.MapMatchedLongitude = lon
	for _, n := range names {
		r.PublicChargingPoint.PotentialChargingPointMatches = append(r.PublicChargingPoint.PotentialChargingPointMatches,
			struct {
				ProviderName string `json:"providerName"`
			}{n})
	}
	return r
}

func TestProcessRawSessionsProviderVoteAndLocationTieBreak(t *testing.T) {
	m := NewManager()
	err := m.ProcessRawSessions([]RawSession{
		// Clear majority.
		rawWithMatches(1000, 48.1001, 11.5001, "HPC Fastned", "HPC IONITY", "HPC/DC IONITY GmbH"),
		// Tie, same place as a clear IONITY session: location decides.
		rawWithMatches(2000, 48.1002, 11.5002, "HPC Fastned", "HPC/DC IONITY"),
		// Tie at a place with no other session: first match stays, flagged.
		rawWithMatches(3000, 50.0, 8.0, "AC Allego", "AC reev"),
		// No match.
		rawWithMatches(4000, 0, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	s := m.GetSessions()
	want := []struct {
		provider  string
		ambiguous bool
	}{{"IONITY", false}, {"IONITY", false}, {"Allego", true}, {UnknownProvider, false}}
	for i, w := range want {
		if s[i].Provider != w.provider || s[i].ProviderAmbiguous != w.ambiguous {
			t.Errorf("session %d: got %q ambiguous=%v, want %q ambiguous=%v", i, s[i].Provider, s[i].ProviderAmbiguous, w.provider, w.ambiguous)
		}
	}
}

func TestGroupProvidersShowsOnlyCleanOperators(t *testing.T) {
	m := NewManager()
	sessions := []Session{
		{Provider: "HPC IONITY"}, {Provider: "IONITY"}, {Provider: "HPC/DC IONITY GmbH", Failed: true},
		{Provider: "Unknown"}, {Provider: ""}, {Provider: "Hauptstraße 12"},
		{Provider: "AC Stadtwerke München GmbH"},
	}
	for i := range sessions {
		sessions[i].EnergyFromGrid = 10
	}
	m.SetSessions(sessions)
	res := m.GroupProviders()
	all := res["all_providers"].([]ProviderStats)
	if len(all) != 2 {
		t.Fatalf("got %d providers %+v, want 2", len(all), all)
	}
	if all[0].Provider != "IONITY" || all[0].Total != 3 || all[0].FailedCount != 1 {
		t.Errorf("got %+v, want IONITY total 3 failed 1", all[0])
	}
	if all[1].Provider != "Stadtwerke München" {
		t.Errorf("got %q, want Stadtwerke München", all[1].Provider)
	}
	for _, k := range []string{"unknown_providers_debug", "provider_matches"} {
		if _, ok := res[k]; ok {
			t.Errorf("response must not contain %s", k)
		}
	}
}
