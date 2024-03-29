package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type ammo struct {
	Name          string
	Flesh         string
	Penetration   int
	Armor         int
	Accuracy      int
	Recoil        int
	Fragmentation int
	Efficency     []int
}

type caliber struct {
	Name  string
	Ammos []*ammo
}

func generateList(data [][]string) []*caliber {
	var ret []*caliber

	for _, line := range data {
		log.Println(line)

		if len(line) > 15 {
			newAmmo := &ammo{line[1],
				line[2],
				safeIntReader(line[3]),
				safeIntReader(line[4]),
				safeIntReader(line[5]),
				safeIntReader(line[6]),
				safeIntReader(strings.Replace(line[7], "%", "", 1)),
				[]int{
					safeIntReader(line[10]),
					safeIntReader(line[11]),
					safeIntReader(line[12]),
					safeIntReader(line[13]),
					safeIntReader(line[14]),
					safeIntReader(line[15]),
				},
			}

			added := false
			for _, c := range ret {
				if c.Name == line[0] {
					c.Ammos = append(c.Ammos, newAmmo)
					added = true
				}
			}
			if !added {
				ret = append(ret, &caliber{line[0], []*ammo{newAmmo}})
			}
		}

	}
	return ret
}

func safeIntReader(s string) int {
	p, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return p
}

const startTable = "{|"
const endTable = "|}"
const newLine = "|-"

func readTable(table string) [][]string {
	ret := [][]string{}
	retLine := []string{}
	posCount := 0
	spanBuffer := make(map[int]int)
	lines := strings.Split(table, "\n")

	linesBuffer := []string{}
	for _, line := range lines {
		if len(linesBuffer) > 1 && strings.HasPrefix(line, newLine) && strings.HasPrefix(linesBuffer[len(linesBuffer)-1], newLine) {
			log.Println("Line removed")
		} else {
			linesBuffer = append(linesBuffer, line)
		}
	}

	for _, line := range linesBuffer {
		spanCount, span := spanBuffer[posCount]
		for span && spanCount > 0 {
			retLine = append(retLine, ret[len(ret)-1][posCount])
			spanBuffer[posCount]--
			posCount++
			spanCount, span = spanBuffer[posCount]
		}
		if strings.HasPrefix(line, startTable) {
		} else if strings.HasPrefix(line, endTable) {
			ret = append(ret, retLine)
		} else if strings.HasPrefix(line, newLine) {
			ret = append(ret, retLine)
			posCount = 0
			retLine = []string{}
		} else if strings.HasPrefix(line, "|") {
			lastDiv := strings.LastIndex(line, "|") + 1
			cellValue := cleanCell(line[lastDiv:])
			if strings.Contains(line, "rowspan") {
				i := extractSpanValue(line)
				if i > 1 {
					spanBuffer[posCount] = i - 1
				}
			}
			if posCount == 1 {
				cellValue = simplifyAmmoName(cellValue)
			}
			retLine = append(retLine, cellValue)
			posCount++
		}

	}
	return ret
}

var reSpan = regexp.MustCompile(`span\=\"([0-9]+)\"`)

func extractSpanValue(s string) int {
	res := reSpan.FindStringSubmatch(s)
	if len(res) == 2 {
		i, err := strconv.Atoi(res[1])
		if err != nil {
			log.Fatalf("ERROR reading rowspan value (%s) line: %s", res[1], s)
		} else {
			return i
		}
	}
	return 1
}

var reSquare = regexp.MustCompile(`\[\[(.*)\]\]`)
var reHTMLTaggo = regexp.MustCompile(`<[^>]*>`)

func cleanCell(s string) string {
	ret := s
	res := reSquare.FindStringSubmatch(s)
	if len(res) == 2 {
		ret = res[1]
	}
	ret = strings.Replace(ret, "]]**", "", 1)
	return reHTMLTaggo.ReplaceAllLiteralString(ret, "")
}

func extractTables(page string) []string {
	ret := []string{}
	for strings.Contains(page, startTable) {
		iStart := strings.Index(page, startTable)
		if iStart >= 0 {
			iEnd := strings.Index(page, endTable)
			if iEnd == -1 {
				log.Fatalln("Missing table end, file might be corrupted.")
			} else {
				iEnd += 2
				ret = append(ret, page[iStart:iEnd])
				page = page[iEnd:]
			}
		}
	}
	return ret
}

var uselessName = [...]string{
	"12/70", "12x70 shell with", "12x70", "20/70", "20x70", "23x75mm", "9x18 PM mm", "9x18 mm PM",
	"7.62x25mm TT", "TT 9x19 mm", ".45 ACP", "9x21 mm", "5.7x28 mm", "4.6x30mm", "9x39 mm", ".366",
	"5.45x39 mm", "5.56x45 mm", "7.62x39 mm", "7.62x51 mm", "7.62x54R", "12.7x55 mm", "40x46 mm"}

func simplifyAmmoName(value string) string {
	for _, un := range uselessName {
		if strings.Contains(value, un) {
			value = strings.ReplaceAll(value, un, "")
		}
	}
	return strings.TrimSpace(value)
}

func main() {
	url := "https://escapefromtarkov.fandom.com/wiki/Ballistics?action=raw"
	log.Printf("GET %s ...\n", url)
	resp, err := http.Get(url)

	if err != nil {
		panic(err)
	}

	// close html request
	defer resp.Body.Close()

	// parse html
	fullPage, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	// Extract a list of tables from page
	tables := extractTables(string(fullPage))

	rawData := readTable(tables[3])
	data := generateList(rawData)

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("serialize error: %s", err)
	}

	err = ioutil.WriteFile("docs/data.json", jsonData, 0644)
	if err != nil {
		log.Fatalf("write file error: %s", err)
	}

	log.Println("Exit 0")
}
