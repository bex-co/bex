// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bex_client_ip_headers

import (
	"bytes"
	"compress/gzip"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixtureCountries is DB-IP's IP to Country Lite (https://db-ip.com, CC BY
// 4.0) in its own format, cut to the rows around a few known addresses
// (dbip-country-lite-2026-11). 8.8.235.255-46.3.255.255 is left out on
// purpose, a gap, and the IPv4 table ends early, at 198.51.100.255.
const fixtureCountries = `0.0.0.0,0.255.255.255,ZZ
1.0.0.0,1.0.0.255,AU
1.0.1.0,8.7.244.255,CN
8.7.245.0,8.8.235.255,US
46.4.0.0,46.5.255.255,DE
46.6.0.0,198.51.99.255,GB
198.51.100.0,198.51.100.255,ZZ
::,1fff:ffff:ffff:ffff:ffff:ffff:ffff:ffff,ZZ
2000::,2001:1ff:ffff:ffff:ffff:ffff:ffff:ffff,US
2001:200::,2001:200:134:ffff:ffff:ffff:ffff:ffff,JP
2001:200:135::,2a01:4f7:ffff:ffff:ffff:ffff:ffff:ffff,NL
2a01:4f8::,2a01:4f8:fff2:6:ffff:ffff:ffff:ffff,DE
2a01:4f8:fff2:7::,ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff,ZZ
`

// w4/m687: CF-IPCountry is the client's ISO 3166-1 alpha-2 country, and XX
// wherever the table has none: DB-IP's ZZ, a gap, past its last range.
func TestCountryLookup(t *testing.T) {
	table, err := parseCountries(strings.NewReader(fixtureCountries))
	if err != nil {
		t.Fatal(err)
	}
	for ip, want := range map[string]string{
		"8.8.8.8":                             "US",
		"8.7.245.0":                           "US",
		"8.7.244.255":                         "CN",
		"46.4.0.1":                            "DE",
		"46.5.255.255":                        "DE",
		"46.6.0.0":                            "GB",
		"1.0.0.0":                             "AU",
		"0.0.0.1":                             "XX",
		"8.8.236.0":                           "XX", // the gap
		"46.3.255.255":                        "XX",
		"198.51.100.20":                       "XX", // ZZ
		"198.51.101.0":                        "XX", // after the last range
		"255.255.255.255":                     "XX",
		"2001:200::1":                         "JP",
		"2a01:4f8::1":                         "DE",
		"2a01:4f8:fff2:6:ffff:ffff:ffff:ffff": "DE",
		"2a01:4f8:fff2:7::":                   "XX",
		"2a01:4f7::1":                         "NL",
		"::1":                                 "XX",
		"::ffff:46.4.0.1":                     "DE", // v4-mapped reads as IPv4
	} {
		if got := table.lookup(net.ParseIP(ip)); got != want {
			t.Errorf("%s = %s, want %s", ip, got, want)
		}
	}
	setCountries(nil)
	if got := country(net.ParseIP("46.4.0.1")); got != "XX" {
		t.Errorf("with no table loaded 46.4.0.1 = %s, want XX", got)
	}
}

// A table the lookup cannot binary-search is refused whole, so the one
// already loaded stays.
func TestCountryTableRefusesABrokenFile(t *testing.T) {
	for name, csv := range map[string]string{
		"out of order":     "1.0.0.0,1.0.0.255,AU\n0.0.0.0,0.255.255.255,ZZ\n::,ffff::,ZZ\n",
		"overlapping":      "0.0.0.0,1.0.0.10,ZZ\n1.0.0.0,1.0.0.255,AU\n::,ffff::,ZZ\n",
		"end before start": "1.0.0.255,1.0.0.0,AU\n::,ffff::,ZZ\n",
		"past the end":     "0.0.0.0,255.255.255.255,ZZ\n1.0.0.0,1.0.0.255,AU\n::,ffff::,ZZ\n",
		"mixed families":   "0.0.0.0,::1,ZZ\n::,ffff::,ZZ\n",
		"not an address":   "0.0.0.0,1.2.3,ZZ\n::,ffff::,ZZ\n",
		"short line":       "0.0.0.0,1.0.0.0\n::,ffff::,ZZ\n",
		"no IPv6":          "0.0.0.0,255.255.255.255,ZZ\n",
		"no IPv4":          "::,ffff::,ZZ\n",
		"html":             "<html>moved</html>\n",
	} {
		if _, err := parseCountries(strings.NewReader(csv)); err == nil {
			t.Errorf("%s: parsed, want an error", name)
		}
	}
	table, err := parseCountries(strings.NewReader("0.0.0.0,1.0.0.0,de\n1.0.0.1,255.255.255.255,EUR\n::,ffff::,ZZ\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a, b := table.lookup(net.ParseIP("0.0.0.1")), table.lookup(net.ParseIP("2.0.0.0")); a != "XX" || b != "XX" {
		t.Errorf("malformed codes read %s and %s, want XX", a, b)
	}
}

// countryServer serves fixtureCountries gzipped for the given months and 404
// for any other, counting requests.
type countryServer struct {
	mu       sync.Mutex
	months   map[string]bool
	requests []string
}

func (s *countryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	month := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/dbip-country-lite-"), ".csv.gz")
	s.requests = append(s.requests, month)
	if !s.months[month] {
		http.NotFound(w, r)
		return
	}
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	_, _ = z.Write([]byte(fixtureCountries))
	_ = z.Close()
	_, _ = w.Write(b.Bytes())
}

func (s *countryServer) take() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.requests
	s.requests = nil
	return r
}

// The loader takes the month's table, the previous month's while DB-IP has
// not published the new one, and keeps what it has when a download fails.
func TestCountryTableRefresh(t *testing.T) {
	srv := &countryServer{months: map[string]bool{"2026-10": true}}
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	saved := countryTableURL
	countryTableURL = server.URL + "/dbip-country-lite-%s.csv.gz"
	t.Cleanup(func() { countryTableURL = saved; setCountries(nil) })
	setCountries(nil)

	nov := time.Date(2026, time.November, 1, 0, 30, 0, 0, time.UTC)
	if err := refreshCountriesOnce(nov); err != nil {
		t.Fatal(err)
	}
	if got := srv.take(); strings.Join(got, ",") != "2026-11,2026-10" {
		t.Errorf("requests %q, want November then October", got)
	}
	if tab := currentCountries(); tab == nil || tab.month != "2026-10" || country(net.ParseIP("46.4.0.1")) != "DE" {
		t.Fatalf("table %+v, want October's loaded", tab)
	}
	// October's is loaded and November's is still missing: one request.
	if err := refreshCountriesOnce(nov.Add(24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := srv.take(); strings.Join(got, ",") != "2026-11" {
		t.Errorf("requests %q, want November alone", got)
	}
	srv.mu.Lock()
	srv.months["2026-11"] = true
	srv.mu.Unlock()
	if err := refreshCountriesOnce(nov.Add(48 * time.Hour)); err != nil || currentCountries().month != "2026-11" {
		t.Fatalf("err %v, table %s, want November's", err, currentCountries().month)
	}
	_ = srv.take()
	// November's is current: no request at all.
	if err := refreshCountriesOnce(nov.Add(72 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := srv.take(); len(got) != 0 {
		t.Errorf("requests %q, want none", got)
	}
	// January finds neither its own month nor December's: it fails and keeps
	// November's.
	if err := refreshCountriesOnce(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("no table for January or December, want an error")
	}
	if got := srv.take(); strings.Join(got, ",") != "2027-01,2026-12" || currentCountries().month != "2026-11" {
		t.Errorf("requests %q table %s, want both months tried and November's kept", got, currentCountries().month)
	}
	// The previous month of March 31 is February.
	_ = refreshCountriesOnce(time.Date(2027, time.March, 31, 23, 0, 0, 0, time.UTC))
	if got := srv.take(); strings.Join(got, ",") != "2027-03,2027-02" {
		t.Errorf("requests %q, want March then February", got)
	}
}
