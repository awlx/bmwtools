package data

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// UnknownProvider is stored when a session has no known charge point operator.
const UnknownProvider = "Unknown"

var (
	// BMW adds the charge point type before the operator, with a slash or a
	// space between the parts: "HPC IONITY", "HPC/DC/AC EnBW mobility+".
	chargeTypePrefixRe = regexp.MustCompile(`(?i)^(?:(?:hpc|dc|ac)(?:\s*/\s*|\s+))+`)
	chargeTypeSuffixRe = regexp.MustCompile(`(?i)\s+(?:hpc|dc|ac)$`)
	// Country or role tags at the end: "Engie Vianeo (FR)", "NewMotion (Germany)".
	tagSuffixRe = regexp.MustCompile(`(?i)\s*\((?:[a-z]{2}|cpo|germany|deutschland|austria|österreich|france|italia|italy|spain|españa)\)$`)

	legalSuffixRe = regexp.MustCompile(`(?i)[\s,]+(?:` + strings.Join([]string{
		`gmbh\s*(?:&|und)\s*co\.?\s*kg`,
		`&\s*co\.?\s*kg`,
		`unipessoal\s+lda`,
		`aktiengesellschaft`,
		`gmbh`, `mbh`, `ag`, `se`, `kg`, `eg`, `ku`, `aör`,
		`b\.\s?v\.?`, `bv`, `n\.\s?v\.?`, `nv`,
		`ltd\.?`, `limited`, `llc`, `inc\.?`, `plc`,
		`s\.\s?a\.?`, `sa`, `s\.\s?l\.?`, `sl`, `s\.r\.l\.?`, `srl`, `s\.p\.a\.?`, `spa`,
		`sasu`, `sas`, `sàrl`, `sarl`,
		`aps`, `a/s`, `ab`, `as`, `oy`, `lda`,
		`d\.o\.o\.?`, `d\.d\.?`, `a\.\s?s\.?`, `s\.r\.o\.?`, `sp\.?\s?z\s?o\.\s?o\.?`,
		`e\.\s?v\.?`, `ood`, `nyrt`, `cpo`,
	}, "|") + `)$`)

	spaceRe      = regexp.MustCompile(`\s+`)
	postalCodeRe = regexp.MustCompile(`\b\d{4,5}\b`)
	streetRe     = regexp.MustCompile(`(?i)(straße|strasse|str\.|weg|platz|allee|gasse|road|street|rue|via)\s*\d`)
)

// Names that BMW sends in the provider field but that are not operators.
var nonProviderNames = map[string]bool{
	"": true, "unknown": true, "unbekannt": true, "n/a": true, "na": true, "none": true,
	"null": true, "other": true, "-": true, "hpc": true, "dc": true, "ac": true,
	"charging operator": true, "charge point operator": true, "cpo": true,
	"aucun": true, "aucun / none": true, "keine": true, "independent": true,
}

// providerAliases maps the lower-case clean name (or its first words) of an
// operator to one display name. A rule matches the full name or a name that
// starts with the rule and a space or a hyphen.
var providerAliases = []struct{ prefix, name string }{
	{"enbw", "EnBW"},
	{"ionity", "IONITY"},
	{"tesla", "Tesla"},
	{"e.on", "E.ON"}, {"e-on", "E.ON"}, {"e on", "E.ON"}, {"eon", "E.ON"},
	{"mer", "Mer"},
	{"chargepoint", "ChargePoint"},
	{"kaufland", "Kaufland"},
	{"lidl", "Lidl"},
	{"aldi süd", "Aldi Süd"},
	{"aldi nord", "Aldi Nord"},
	{"aldi", "ALDI"},
	{"bp", "BP"},
	{"omv", "OMV"},
	{"mol", "MOL Plugee"},
	{"tiwag", "TIWAG"},
	{"atlante", "Atlante"},
	{"mennekes", "Mennekes"},
	{"sortimo", "Sortimo"},
	{"mblty", "mblty"},
	{"ladenetz", "ladenetz.de"}, {"ladenetz.de", "ladenetz.de"},
	{"engie vianeo", "Engie Vianeo"},
	{"newmotion", "NewMotion"},
	{"comfort charge", "Comfortcharge"},
	{"ok", "OK"},
	{"totalenergies", "TotalEnergies"}, {"total", "TotalEnergies"},
	{"fastned", "Fastned"},
	{"virta", "Virta"},
	{"vattenfall", "Vattenfall"},
	{"endesa", "Endesa"},
	{"enel", "Enel"},
	{"innogy", "innogy"},
	{"neogy", "Neogy"},
	{"repsol", "Repsol"},
	{"powerdot", "Powerdot"}, {"power dot", "Powerdot"},
	{"porsche", "Porsche"},
	{"ikea", "IKEA"},
	{"petrol", "Petrol"},
	{"izivia", "IZIVIA"},
	{"qwello", "Qwello"},
	{"audi", "Audi"},
	{"shell", "Shell Recharge"},
	{"compleo", "Compleo"},
	{"allego", "Allego"},
	{"smatrics", "SMATRICS"},
	{"ewe go", "EWE Go"},
	{"aral pulse", "Aral pulse"},
	{"moon power", "Moon Power"},
	{"electra", "Electra"},
	{"greenflux", "Greenflux"},
	{"monta", "Monta"},
	{"jolt", "Jolt"},
	{"circle k", "Circle K"},
	{"edeka", "Edeka"},
	{"pfalzwerke", "Pfalzwerke"},
	{"comfortcharge", "Comfortcharge"},
	{"iberdrola", "Iberdrola"},
	{"move mobility", "MOVE Mobility"}, {"groupe e / move mobility", "MOVE Mobility"},
	{"da emobil", "da emobil"},
}

// CleanProviderName returns the display name of the charge point operator, or
// "" when the text does not name an operator. It removes the charge point type
// (HPC/DC/AC), legal suffixes and surplus spaces, and maps known aliases to
// one name.
func CleanProviderName(raw string) string {
	name := strings.TrimSpace(spaceRe.ReplaceAllString(raw, " "))
	if nonProviderNames[strings.ToLower(name)] {
		return ""
	}
	name = chargeTypePrefixRe.ReplaceAllString(name, "")
	for {
		next := tagSuffixRe.ReplaceAllString(name, "")
		next = chargeTypeSuffixRe.ReplaceAllString(next, "")
		next = legalSuffixRe.ReplaceAllString(next, "")
		next = strings.TrimRight(strings.TrimSpace(next), ",-/&")
		next = strings.TrimSpace(next)
		if next == name || letterCount(next) < 2 {
			break
		}
		name = next
	}

	key := strings.ToLower(name)
	if nonProviderNames[key] || letterCount(name) < 2 || looksLikeAddress(name) {
		return ""
	}
	for _, a := range providerAliases {
		if key == a.prefix || strings.HasPrefix(key, a.prefix+" ") || strings.HasPrefix(key, a.prefix+"-") {
			return a.name
		}
	}
	return name
}

// ProviderKey is the grouping key for a clean provider name.
func ProviderKey(clean string) string {
	return strings.ToLower(clean)
}

func letterCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

func looksLikeAddress(s string) bool {
	if streetRe.MatchString(s) {
		return true
	}
	return strings.Contains(s, ",") && postalCodeRe.MatchString(s)
}

// providerVote is the result of the majority vote over the charge point
// matches of one session.
type providerVote struct {
	winner string   // display name; "" when no match names an operator
	tied   []string // display names with the same top count, in match order
}

// voteProvider counts the clean operator names of all matches. The most
// frequent name wins. On a tie, all tied names are returned in match order.
func voteProvider(matches []string) providerVote {
	counts := make(map[string]int)
	names := make(map[string]string)
	var order []string
	for _, m := range matches {
		name := CleanProviderName(m)
		if name == "" {
			continue
		}
		key := ProviderKey(name)
		if counts[key] == 0 {
			order = append(order, key)
			names[key] = name
		}
		counts[key]++
	}
	best := 0
	for _, k := range order {
		if counts[k] > best {
			best = counts[k]
		}
	}
	var tied []string
	for _, k := range order {
		if counts[k] == best {
			tied = append(tied, names[k])
		}
	}
	if len(tied) == 0 {
		return providerVote{}
	}
	if len(tied) == 1 {
		return providerVote{winner: tied[0]}
	}
	return providerVote{winner: tied[0], tied: tied}
}

// locationKey groups sessions at the same place (about 100 m).
func locationKey(lat, lon float64) string {
	if lat == 0 && lon == 0 {
		return ""
	}
	return fmt.Sprintf("%.3f,%.3f", math.Round(lat*1000)/1000, math.Round(lon*1000)/1000)
}

// resolveProviderTies breaks majority-vote ties with the operator that wins
// most often at the same location in the same upload. When that does not
// decide, the first tied match stays. Tied sessions get ProviderAmbiguous.
func resolveProviderTies(sessions []Session, votes []providerVote) {
	atLocation := make(map[string]map[string]int)
	for i, v := range votes {
		loc := locationKey(sessions[i].Latitude, sessions[i].Longitude)
		if v.winner == "" || len(v.tied) > 0 || loc == "" {
			continue
		}
		if atLocation[loc] == nil {
			atLocation[loc] = make(map[string]int)
		}
		atLocation[loc][ProviderKey(v.winner)]++
	}
	for i, v := range votes {
		if len(v.tied) == 0 {
			continue
		}
		sessions[i].ProviderAmbiguous = true
		counts := atLocation[locationKey(sessions[i].Latitude, sessions[i].Longitude)]
		best, bestCount, unique := v.tied[0], -1, false
		for _, name := range v.tied {
			c := counts[ProviderKey(name)]
			switch {
			case c > bestCount:
				best, bestCount, unique = name, c, true
			case c == bestCount:
				unique = false
			}
		}
		if bestCount > 0 && unique {
			sessions[i].Provider = best
			sessions[i].ProviderAmbiguous = false
		}
	}
}
