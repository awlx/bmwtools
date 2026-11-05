package vehicles

import "testing"

func TestGroupsShareOnePackPerOption(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Catalog() {
		sizes := map[float64]bool{}
		for _, g := range f.Groups {
			if g.Family != f.Name {
				t.Errorf("%s: family %q", g.Value, g.Family)
			}
			if seen[g.Value] {
				t.Errorf("duplicate value %q", g.Value)
			}
			seen[g.Value] = true
			if sizes[g.NetKWh] {
				t.Errorf("%s: two options with the same pack size %.1f kWh", f.Name, g.NetKWh)
			}
			sizes[g.NetKWh] = true
		}
	}
}

func TestI4Groups(t *testing.T) {
	groups := GroupsOf("BMW i4")
	if len(groups) != 2 {
		t.Fatalf("i4 groups = %+v", groups)
	}
	if groups[1].Label != "BMW i4 eDrive40 / xDrive40 / M50 / M60 (80.7 kWh)" {
		t.Errorf("label = %q", groups[1].Label)
	}
	if ce := GroupsOf("BMW CE 04"); len(ce) != 1 || ce[0].Label != "BMW CE 04 (8.9 kWh)" {
		t.Errorf("CE 04 = %+v", ce)
	}
	if g := GroupsOf("Mini Cooper"); g[0].Label != "Mini Cooper SE (F56) (28.9 kWh)" {
		t.Errorf("Mini label = %q", g[0].Label)
	}
	if g := GroupsOf("BMW i3"); g[0].Label != "BMW i3 / i3s 60 Ah (18.8 kWh)" {
		t.Errorf("i3 label = %q", g[0].Label)
	}
	if groups[0].Trims[0] != "eDrive35" {
		t.Errorf("first i4 group = %+v", groups[0])
	}
}

func TestFamilyOfAndValidation(t *testing.T) {
	cases := map[string]string{
		"BMW i4 80.7 kWh":   "BMW i4",
		"BMW i4":            "BMW i4",
		"BMW iX3 108.7 kWh": "BMW iX3",
		"Mini Cooper SE":    "Mini Cooper",
		"Something else":    "Something else",
	}
	for in, want := range cases {
		if got := FamilyOf(in); got != want {
			t.Errorf("FamilyOf(%q) = %q, want %q", in, got, want)
		}
	}
	for _, ok := range []string{"BMW i4 67.1 kWh", "BMW i4", "Mini Cooper SE", "Other BMW EV", "BMW CE 04"} {
		if !IsValidUploadModel(ok) {
			t.Errorf("%q must be valid", ok)
		}
	}
	for _, bad := range []string{"", "BMW i4 99 kWh", "<script>"} {
		if IsValidUploadModel(bad) {
			t.Errorf("%q must be invalid", bad)
		}
	}
}
