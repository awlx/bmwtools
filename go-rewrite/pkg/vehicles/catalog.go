// Package vehicles holds the catalog of BMW group EVs offered at upload time.
//
// Upload options are battery-size groups, not single trims: trims that share
// one pack (e.g. i4 eDrive40, xDrive40, M50 and its facelift name M60) form one
// group. Facelift renames that changed the pack (e.g. iX xDrive50 → xDrive60)
// are separate groups. The stored model value of a group is stable; its label
// may change when trims are added.
package vehicles

import (
	"fmt"
	"strings"
)

// PackGroup is one selectable battery-size group of a model family.
type PackGroup struct {
	Value      string   `json:"value"`
	Family     string   `json:"family"`
	Generation string   `json:"generation,omitempty"`
	Trims      []string `json:"trims"`
	NetKWh     float64  `json:"net_kwh"`
	Label      string   `json:"label"`
}

// Family is a model family with its battery-size groups. A family without
// groups (e.g. "Other BMW EV") can be selected directly.
type Family struct {
	Name   string      `json:"name"`
	Groups []PackGroup `json:"groups"`
}

func group(family, generation string, netKWh float64, trims ...string) PackGroup {
	return newGroup(family, generation, netKWh, trims)
}

// groupLabel avoids repeated words, e.g. "Mini Cooper" + "Cooper SE" gives
// "Mini Cooper SE", and "BMW CE 04" + "CE 04" gives "BMW CE 04".
func groupLabel(family string, netKWh float64, trims []string) string {
	joined := strings.Join(trims, " / ")
	switch {
	case strings.HasSuffix(family, " "+joined):
		joined = ""
	default:
		words := strings.Fields(family)
		last := words[len(words)-1]
		if strings.HasPrefix(joined, last+" ") {
			joined = strings.TrimPrefix(joined, last+" ")
		}
	}
	if joined == "" {
		return fmt.Sprintf("%s (%.1f kWh)", family, netKWh)
	}
	return fmt.Sprintf("%s %s (%.1f kWh)", family, joined, netKWh)
}

func newGroup(family, generation string, netKWh float64, trims []string) PackGroup {
	label := groupLabel(family, netKWh, trims)
	return PackGroup{
		Value:      fmt.Sprintf("%s %.1f kWh", family, netKWh),
		Family:     family,
		Generation: generation,
		Trims:      trims,
		NetKWh:     netKWh,
		Label:      label,
	}
}

// Net (usable) capacities from manufacturer data.
var catalog = []Family{
	{Name: "BMW i3", Groups: []PackGroup{
		group("BMW i3", "", 18.8, "i3 / i3s 60 Ah"),
		group("BMW i3", "", 27.2, "i3 / i3s 94 Ah"),
		group("BMW i3", "", 37.9, "i3 / i3s 120 Ah"),
	}},
	{Name: "BMW i4", Groups: []PackGroup{
		group("BMW i4", "", 67.1, "eDrive35"),
		group("BMW i4", "", 80.7, "eDrive40", "xDrive40", "M50", "M60"),
	}},
	{Name: "BMW i5", Groups: []PackGroup{
		group("BMW i5", "Sedan and Touring", 81.2, "eDrive40", "xDrive40", "M60"),
	}},
	{Name: "BMW i7", Groups: []PackGroup{
		group("BMW i7", "", 101.7, "eDrive50", "xDrive60", "M70"),
	}},
	{Name: "BMW iX", Groups: []PackGroup{
		group("BMW iX", "2021–2025", 71.0, "xDrive40"),
		group("BMW iX", "2025+", 94.8, "xDrive45"),
		group("BMW iX", "2021–2025", 105.2, "xDrive50", "M60"),
		group("BMW iX", "2025+", 109.1, "xDrive60", "M70"),
	}},
	{Name: "BMW iX1", Groups: []PackGroup{
		group("BMW iX1", "", 64.7, "eDrive20", "xDrive30"),
	}},
	{Name: "BMW iX2", Groups: []PackGroup{
		group("BMW iX2", "", 64.7, "eDrive20", "xDrive30"),
	}},
	{Name: "BMW iX3", Groups: []PackGroup{
		group("BMW iX3", "G08, 2020–2025", 74.0, "G08"),
		group("BMW iX3", "Neue Klasse, 2025+", 82.6, "Neue Klasse 40"),
		group("BMW iX3", "Neue Klasse, 2025+", 108.7, "Neue Klasse 50 xDrive"),
	}},
	{Name: "Mini Cooper", Groups: []PackGroup{
		group("Mini Cooper", "F56, 2020–2024", 28.9, "Cooper SE (F56)"),
		group("Mini Cooper", "J01, 2024+", 36.6, "Cooper E (J01)"),
		group("Mini Cooper", "J01, 2024+", 49.2, "Cooper SE (J01)"),
	}},
	{Name: "Mini Aceman", Groups: []PackGroup{
		group("Mini Aceman", "", 38.5, "E"),
		group("Mini Aceman", "", 49.2, "SE"),
	}},
	{Name: "Mini Countryman", Groups: []PackGroup{
		group("Mini Countryman", "", 64.6, "E", "SE ALL4"),
	}},
	{Name: "BMW CE 04", Groups: []PackGroup{
		group("BMW CE 04", "Scooter", 8.9, "CE 04"),
	}},
	{Name: "BMW CE 02", Groups: []PackGroup{
		group("BMW CE 02", "Scooter", 3.9, "CE 02"),
	}},
	{Name: "Other BMW EV"},
}

// legacyFamilies maps model values stored before pack groups existed.
var legacyFamilies = map[string]string{
	"Mini Cooper SE": "Mini Cooper",
	"Unknown BMW EV": "Other BMW EV",
}

var byValue = func() map[string]PackGroup {
	m := map[string]PackGroup{}
	for _, f := range catalog {
		for _, g := range f.Groups {
			m[g.Value] = g
		}
	}
	return m
}()

// Catalog returns all families with their groups.
func Catalog() []Family {
	out := make([]Family, len(catalog))
	copy(out, catalog)
	return out
}

// Group returns the pack group for a stored model value.
func Group(model string) (PackGroup, bool) {
	g, ok := byValue[model]
	return g, ok
}

// FamilyOf returns the family of a stored model value. Legacy family-only
// values map to their family; unknown values are their own family.
func FamilyOf(model string) string {
	if g, ok := byValue[model]; ok {
		return g.Family
	}
	if f, ok := legacyFamilies[model]; ok {
		return f
	}
	return model
}

// GroupsOf returns the pack groups of a family.
func GroupsOf(family string) []PackGroup {
	for _, f := range catalog {
		if f.Name == family {
			return f.Groups
		}
	}
	return nil
}

// IsValidUploadModel reports whether model is a known group value, a family
// name, or a legacy value (older cached pages may still send these).
func IsValidUploadModel(model string) bool {
	if _, ok := byValue[model]; ok {
		return true
	}
	if _, ok := legacyFamilies[model]; ok {
		return true
	}
	for _, f := range catalog {
		if f.Name == model {
			return true
		}
	}
	return false
}
