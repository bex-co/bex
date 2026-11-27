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
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// CF-IPCountry (w4/m687): Cloudflare tells a Render app the client's country
// as an ISO 3166-1 alpha-2 code, XX when it does not know. The edge reads it
// from DB-IP's IP to Country Lite database (https://db-ip.com, CC BY 4.0,
// IPv4 and IPv6, about 4.6 MB gzipped and 730,000 ranges in 2026-11), which
// DB-IP publishes monthly. Too large for a ConfigMap or the repository, it is
// downloaded by the plugin itself in the background: at Traefik's start, then
// every countryRefresh until the month's table is loaded, so a new month
// arrives without a Traefik release. Until a table loads every address reads
// XX, and a failed download keeps the table already loaded.

// countryTableURL is DB-IP's monthly download; %s is the month, as 2006-01.
var countryTableURL = "https://download.db-ip.com/free/dbip-country-lite-%s.csv.gz"

var (
	// countryRefresh is how often the loader looks for the month's table.
	countryRefresh = 24 * time.Hour
	// countryRetry is how soon it tries again after a failed download.
	countryRetry = 10 * time.Minute
	// countryTimeout bounds one download.
	countryTimeout = 5 * time.Minute
)

// maxCountryTable bounds the decompressed table (32 MB in 2026-11).
const maxCountryTable = 256 << 20

// unknownCountry is Cloudflare's code for an address it cannot place.
const unknownCountry = "XX"

// countryTable is the two families' tables: the starts of contiguous ranges
// in ascending order, each with its index into codes. A range runs to the
// next start; the last to the end of the family's space. Plain slices and
// inlined comparisons, because Traefik interprets the plugin: a function call
// or method value per comparison made the lookup's p99 five times slower.
type countryTable struct {
	month      string
	codes      []string
	v4         []uint32
	v4code     []uint16
	v6hi, v6lo []uint64
	v6code     []uint16
}

// countries is the process-wide table every instance reads. Traefik loads a
// plugin into one interpreter, so the package state is shared by the
// entrypoint's instances on every router.
var countries struct {
	once  sync.Once
	mu    sync.RWMutex
	table *countryTable
}

// startCountries starts the background loader once per process.
func startCountries() {
	countries.once.Do(func() { go refreshCountries() })
}

func currentCountries() *countryTable {
	countries.mu.RLock()
	defer countries.mu.RUnlock()
	return countries.table
}

func setCountries(t *countryTable) {
	countries.mu.Lock()
	countries.table = t
	countries.mu.Unlock()
}

// country is CF-IPCountry for a client address.
func country(ip net.IP) string {
	t := currentCountries()
	if t == nil {
		return unknownCountry
	}
	return t.lookup(ip)
}

// refreshCountries keeps the table current for the life of the process.
func refreshCountries() {
	for {
		wait := countryRefresh
		if err := refreshCountriesOnce(time.Now()); err != nil {
			log.Printf("bex-client-ip-headers: CF-IPCountry table: %v (retrying in %v)", err, countryRetry)
			wait = countryRetry
		}
		time.Sleep(wait)
	}
}

// refreshCountriesOnce loads now's month's table, or the previous month's
// while DB-IP has not published the new one. A panic is an error: the
// interpreter would otherwise take the whole edge down with it.
func refreshCountriesOnce(now time.Time) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	now = now.UTC()
	months := []string{now.Format("2006-01"), time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01")}
	var failures []string
	for _, month := range months {
		if t := currentCountries(); t != nil && t.month == month {
			return nil
		}
		t, err := fetchCountries(fmt.Sprintf(countryTableURL, month))
		if err != nil {
			failures = append(failures, month+": "+err.Error())
			continue
		}
		t.month = month
		setCountries(t)
		// Traefik logs a plugin's stdout at debug level, its stderr as errors.
		fmt.Fprintf(os.Stdout, "bex-client-ip-headers: CF-IPCountry table %s loaded: %d IPv4 and %d IPv6 ranges\n", month, len(t.v4), len(t.v6hi))
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

func fetchCountries(url string) (*countryTable, error) {
	client := &http.Client{Timeout: countryTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	z, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseCountries(io.LimitReader(z, maxCountryTable))
}

// parseCountries reads DB-IP's CSV, one "start,end,code" range per line,
// IPv4 then IPv6, each family ascending. DB-IP's ZZ (reserved and unassigned
// space) reads as Cloudflare's XX, and so does any gap between ranges or
// after the last one. A table the lookup could not binary-search is refused.
func parseCountries(r io.Reader) (*countryTable, error) {
	// DB-IP's 2026-11 table has 363,000 IPv4 and 370,000 IPv6 ranges.
	t := &countryTable{
		codes: []string{unknownCountry},
		v4:    make([]uint32, 0, 400000), v4code: make([]uint16, 0, 400000),
		v6hi: make([]uint64, 0, 400000), v6lo: make([]uint64, 0, 400000), v6code: make([]uint16, 0, 400000),
	}
	index := map[string]uint16{unknownCountry: 0}
	// The address after each family's last range, and whether that range
	// reached the end of the family's space.
	var next4, next6hi, next6lo uint64
	var full4, full6 bool
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		fields := strings.Split(sc.Text(), ",")
		if len(fields) != 3 {
			return nil, fmt.Errorf("line %d: want start,end,country", line)
		}
		code := fields[2]
		if code == "ZZ" || len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
			code = unknownCountry
		}
		ci, ok := index[code]
		if !ok {
			ci = uint16(len(t.codes))
			index[code] = ci
			t.codes = append(t.codes, code)
		}
		// The family is the text's: "::ffff:0:0" is an IPv6 row.
		v6 := strings.Contains(fields[0], ":")
		start, end := net.ParseIP(fields[0]), net.ParseIP(fields[1])
		if start == nil || end == nil || strings.Contains(fields[1], ":") != v6 {
			return nil, fmt.Errorf("line %d: not an address range", line)
		}
		if !v6 {
			s, e := uint64(binary.BigEndian.Uint32(start.To4())), uint64(binary.BigEndian.Uint32(end.To4()))
			if full4 || s < next4 || e < s {
				return nil, fmt.Errorf("line %d: ranges overlap or are out of order", line)
			}
			if s > next4 {
				t.v4 = append(t.v4, uint32(next4))
				t.v4code = append(t.v4code, 0)
			}
			t.v4 = append(t.v4, uint32(s))
			t.v4code = append(t.v4code, ci)
			next4, full4 = e+1, e == 1<<32-1
			continue
		}
		start, end = start.To16(), end.To16()
		sHi, sLo := binary.BigEndian.Uint64(start[:8]), binary.BigEndian.Uint64(start[8:])
		eHi, eLo := binary.BigEndian.Uint64(end[:8]), binary.BigEndian.Uint64(end[8:])
		if full6 || sHi < next6hi || sHi == next6hi && sLo < next6lo || eHi < sHi || eHi == sHi && eLo < sLo {
			return nil, fmt.Errorf("line %d: ranges overlap or are out of order", line)
		}
		if sHi != next6hi || sLo != next6lo {
			t.v6hi = append(t.v6hi, next6hi)
			t.v6lo = append(t.v6lo, next6lo)
			t.v6code = append(t.v6code, 0)
		}
		t.v6hi = append(t.v6hi, sHi)
		t.v6lo = append(t.v6lo, sLo)
		t.v6code = append(t.v6code, ci)
		full6 = eHi == ^uint64(0) && eLo == ^uint64(0)
		next6hi, next6lo = eHi, eLo+1
		if eLo == ^uint64(0) {
			next6hi++
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(t.v4) == 0 || len(t.v6hi) == 0 {
		return nil, errors.New("the table has no IPv4 or no IPv6 ranges")
	}
	if !full4 {
		t.v4 = append(t.v4, uint32(next4))
		t.v4code = append(t.v4code, 0)
	}
	if !full6 {
		t.v6hi = append(t.v6hi, next6hi)
		t.v6lo = append(t.v6lo, next6lo)
		t.v6code = append(t.v6code, 0)
	}
	return t, nil
}

// lookup finds the last range starting at or before ip. A v4-mapped IPv6
// address reads as IPv4, as peerIP gives it.
func (t *countryTable) lookup(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		x := binary.BigEndian.Uint32(v4)
		starts := t.v4
		i, j := 0, len(starts)
		for i < j {
			m := int(uint(i+j) >> 1)
			if x < starts[m] {
				j = m
			} else {
				i = m + 1
			}
		}
		if i == 0 {
			return unknownCountry
		}
		return t.codes[t.v4code[i-1]]
	}
	ip = ip.To16()
	if ip == nil {
		return unknownCountry
	}
	hi, lo := binary.BigEndian.Uint64(ip[:8]), binary.BigEndian.Uint64(ip[8:])
	his, los := t.v6hi, t.v6lo
	i, j := 0, len(his)
	for i < j {
		m := int(uint(i+j) >> 1)
		if hi < his[m] || hi == his[m] && lo < los[m] {
			j = m
		} else {
			i = m + 1
		}
	}
	if i == 0 {
		return unknownCountry
	}
	return t.codes[t.v6code[i-1]]
}
