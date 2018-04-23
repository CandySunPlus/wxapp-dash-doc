package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
)

// Entry for doc
type Entry struct {
	basePath string
	file     string
}

func main() {
	docBase := flag.String("docBase", "./miniprogram", "document base path")
	docsetName := flag.String("name", "wxapp", "docset name")
	docsetIcon := flag.String("icon", "./icon.png", "docset icon")
	docsetPath := flag.String("outpath", "./", "output docset path")

	flag.Parse()

	if len(*docBase) <= 0 {
		panic("Can't get doc base path")
	}

	if len(*docsetName) == 0 || len(*docsetPath) == 0 {
		panic("Docset path error")
	}

	docset := path.Join(*docsetPath, *docsetName+".docset")

	initDocset(*docsetName, *docsetIcon, *docBase, docset)

}

func initDocset(name string, icon string, docBase string, docsetPath string) {
	docPath := path.Join(docsetPath, "Contents", "Resources", "Documents")
	dbPath := path.Join(docsetPath, "Contents", "Resources", "docSet.dsidx")
	iconPath := path.Join(docsetPath, "icon.png")
	infoPath := path.Join(docsetPath, "Contents", "info.plist")
	err := os.RemoveAll(docsetPath)
	if err != nil {
		panic("Can't delete old docset")
	}

	err = os.MkdirAll(docPath, 0755)
	if err != nil {
		panic("Can't create docset")
	}

	err = exec.Command("cp", "-rf", path.Join(docBase, "dev"), docPath).Run()
	if err != nil {
		panic("Can't copy documents")
	}
	initDb(docPath, dbPath)
	initInfo(name, infoPath)
	initIcon(icon, iconPath)
}

func initDb(docPath string, dbPath string) {

	db, err := sql.Open("sqlite3", dbPath)

	if err != nil {
		panic("Can't open db file")
	}

	defer db.Close()

	sqlStmt := `CREATE TABLE searchIndex(id INTEGER PRIMARY KEY, name TEXT, type TEXT, path TEXT);
		CREATE UNIQUE INDEX anchor ON searchIndex (name, type, path); `

	db.Exec(sqlStmt)

	tx, err := db.Begin()

	if err != nil {
		return
	}

	entries := []Entry{{"dev/api", "index.html"}, {"dev/component", "index.html"}, {"dev/framework", "MINA.html"}}

	for _, entry := range entries {
		entry.parse(docPath, tx)
	}

	tx.Commit()
}

func initInfo(name string, infoPath string) {
	infoFile, err := os.Create(infoPath)
	if err != nil {
		fmt.Println(err)
		panic("Can't create info file")
	}
	defer infoFile.Close()
	fmt.Fprintf(infoFile, `<?xml version="1.0" encoding="UTF-8"?>
		<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
		<plist version="1.0">
		<dict>
				<key>CFBundleIdentifier</key>
				<string>%s</string>
				<key>CFBundleName</key>
				<string>%s</string>
				<key>DashDocSetFamily</key>
				<string>%s</string>
				<key>DocSetPlatformFamily</key>
				<string>requests</string>
				<key>isDashDocset</key>
				<true/>
				<key>isJavaScriptEnabled</key>
				<true/>
				<key>dashIndexFilePath</key>
				<string>%s</string>
		</dict>
		</plist>`, name, name, name, "dev/framework/MINA.html")
}

func initIcon(icon string, iconPath string) {
	err := exec.Command("cp", icon, iconPath).Run()
	if err != nil {
		panic("Icon copy error")
	}
}

func (entry *Entry) parse(docBase string, tx *sql.Tx) {
	entryPage := path.Join(docBase, entry.basePath, entry.file)

	fmt.Printf("find for %s \n", entryPage)
	entryPageFile, err := os.Open(entryPage)

	if err != nil {
		panic("Can't find index file")
	}

	doc, err := goquery.NewDocumentFromReader(entryPageFile)

	if err != nil {
		panic("index file parse failed")
	}

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO searchIndex(name, type, path) VALUES(?, ?, ?)")

	if err != nil {
		panic(err)
	}

	defer stmt.Close()

	doc.Find("nav a").Each(func(i int, s *goquery.Selection) {
		isTitle := !s.Next().HasClass("articles")
		href, exists := s.Attr("href")
		text := strings.TrimSpace(s.Text())
		if isTitle && exists && len(text) > 0 {
			if !(strings.HasPrefix(href, "http")) {
				stmt.Exec(text, "Section", path.Join(entry.basePath, href))
			}
		}
	})

}
