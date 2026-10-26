package jobsite

import (
	"slices"
	"strings"
)

// 근무지 글자("나주", "서울 강남구", "서울, 경기")를 사이트가 받는 지역 코드로 바꾼다.
// 표(regions_data.go)는 원티드의 지역 목록이고, 시·도마다 사람인 · 점핏의 1차 지역 코드를 붙여 두었다.

type province struct {
	Wanted    string // 원티드 시·도 키 (seoul) — 전체는 seoul.all
	Saramin   string // 사람인 · 점핏 1차 지역 코드 (101000)
	Names     []string
	Districts []district
}

type district struct {
	Wanted string // 원티드 시·군·구 키 (s-jeolla.naju-si)
	Name   string // 나주시
}

// region 은 근무지 글자를 풀어 낸 결과다. 비어 있으면(근무지 없음 · 못 알아봄) 지역 조건을 걸지 않는다.
type region struct {
	wanted  []string // 원티드 locations 값
	saramin []string // 사람인 · 점핏 지역 코드
	// 공고의 근무지 글자와 맞춰 볼 이름 (시·군·구는 그 이름, 시·도 전체는 시·도의 여러 이름)
	words []string
}

func (r region) resolved() bool { return len(r.wanted) > 0 }

// matches 는 공고의 근무지 글자가 이 지역인지 본다 (점핏 '전남광주 나주시' · 사람인 '전남 나주시' 모두 '나주'로).
func (r region) matches(location string) bool {
	return slices.ContainsFunc(r.words, func(w string) bool { return strings.Contains(location, w) })
}

// regionSuffixes 는 비교할 때 떼는 행정 단위 꼬리다 ("나주시" = "나주", "전라남도" ≠ 떼지 않음 — 이름 목록에 따로 있다).
var regionSuffixes = []string{"특별자치시", "특별자치도", "특별시", "광역시", "시", "군", "구"}

// regionWord 는 비교용으로 꼬리를 뗀다. 한 글자만 남으면(중구 → 중) 그대로 둔다.
func regionWord(s string) string {
	s = strings.TrimSpace(s)
	for _, suf := range regionSuffixes {
		if rest, ok := strings.CutSuffix(s, suf); ok && len([]rune(rest)) >= 2 {
			return rest
		}
	}
	return s
}

// splitAlternatives 는 쉼표 · 슬래시로 나눈 근무지 후보들이다. 후보 안의 낱말은 띄어쓰기로 나눈다.
func splitAlternatives(location string) [][]string {
	var out [][]string
	for alt := range strings.FieldsFuncSeq(location, func(r rune) bool { return r == ',' || r == '/' || r == '·' }) {
		if words := strings.Fields(alt); len(words) > 0 {
			out = append(out, words)
		}
	}
	return out
}

// resolveRegion 은 근무지 글자를 지역 코드로 바꾼다.
// 후보끼리는 '또는'(서울, 경기), 후보 안에서는 시·도로 시·군·구를 좁힌다(서울 강서구 ≠ 부산 강서구).
// 시·도 이름과 시·군·구 이름이 같으면 시·도로 본다(광주 → 광주광역시, 경기 광주시는 '경기 광주').
func resolveRegion(location string) region {
	var r region
	for _, words := range splitAlternatives(location) {
		var provs []*province
		var dists []string
		distProv := map[string]*province{}
		distName := map[string]string{}
		for _, w := range words {
			key := regionWord(w)
			// 앞에 시·도가 나왔으면 그 안의 시·군·구부터 (경기 광주 → 경기 광주시)
			if d := findDistricts(provs, key); len(d) > 0 {
				for _, x := range d {
					dists = append(dists, x.d.Wanted)
					distProv[x.d.Wanted] = x.p
					distName[x.d.Wanted] = x.d.Name
				}
				continue
			}
			if p := findProvince(key); len(p) > 0 {
				provs = append(provs, p...)
				continue
			}
			all := make([]*province, len(provinces))
			for i := range provinces {
				all[i] = &provinces[i]
			}
			for _, x := range findDistricts(all, key) {
				dists = append(dists, x.d.Wanted)
				distProv[x.d.Wanted] = x.p
				distName[x.d.Wanted] = x.d.Name
			}
		}
		// 시·도가 있으면 그 안의 시·군·구만
		if len(provs) > 0 && len(dists) > 0 {
			dists = slices.DeleteFunc(dists, func(d string) bool { return !slices.Contains(provs, distProv[d]) })
		}
		if len(dists) > 0 {
			for _, d := range dists {
				r.wanted = appendNew(r.wanted, d)
				r.saramin = appendNew(r.saramin, distProv[d].Saramin)
				r.words = appendNew(r.words, regionWord(distName[d]))
			}
			continue
		}
		for _, p := range provs {
			r.wanted = appendNew(r.wanted, p.Wanted+".all")
			r.saramin = appendNew(r.saramin, p.Saramin)
			for _, n := range p.Names {
				r.words = appendNew(r.words, regionWord(n))
			}
		}
	}
	return r
}

func findProvince(word string) []*province {
	var out []*province
	for i := range provinces {
		if slices.ContainsFunc(provinces[i].Names, func(n string) bool { return regionWord(n) == word }) {
			out = append(out, &provinces[i])
		}
	}
	return out
}

type districtIn struct {
	p *province
	d district
}

func findDistricts(provs []*province, word string) []districtIn {
	var out []districtIn
	for _, p := range provs {
		for _, d := range p.Districts {
			if regionWord(d.Name) == word {
				out = append(out, districtIn{p, d})
			}
		}
	}
	return out
}

func appendNew(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}
