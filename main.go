package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var DB *sql.DB

func init() {
	godotenv.Load()
	var err error
	DB, err = sql.Open("mysql", os.Getenv("DB_URI"))
	if err != nil {
		panic(err)
	}
	if err := DB.Ping(); err != nil {
		panic(err)
	}
	fmt.Printf("DB is connected %v", DB)
}

const (
	staticDir string = "static"
	viewsDir  string = "templates"
)

func main() {
	defer DB.Close()
	http.Handle("/"+staticDir+"/", http.StripPrefix("/"+staticDir+"/", http.FileServer(http.Dir(staticDir))))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticDir+"/favicon.png")
	})
	http.HandleFunc("/", readData)
	http.HandleFunc("/update", updateData)
	http.HandleFunc("/update/call", updateDataCall)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	http.ListenAndServe(":"+port, nil)
	fmt.Printf("\nServer listens on %v", port)
}

type Data struct {
	ID           int    `json:"id"`
	DateAdd      string `json:"dateAdd"`
	ContentValue string `json:"contentValue"`
	ContentType  string `json:"contentType"`
}

func updateData(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "Create a new resources", map[string]any{
		"template": "form",
		"code":     http.StatusOK,
	})
}

func renderTemplate(w http.ResponseWriter, title string, dataTemplate ...map[string]any) {
	templateNameDefault := "result"
	statusCodeDefault := http.StatusBadRequest
	if len(dataTemplate) > 0 {
		dataTpl := dataTemplate[0]
		if dataTpl["template"] != "" {
			templateNameDefault = dataTpl["template"].(string)
		}
		if dataTpl["code"] != "" {
			statusCodeDefault = dataTpl["code"].(int)
		}
	}
	w.WriteHeader(statusCodeDefault)
	tpl, err := template.ParseFiles(
		viewsDir+"/base.html",
		viewsDir+"/includes/"+templateNameDefault+".include.html",
		viewsDir+"/includes/head.include.html",
	)
	if err != nil {
		http.Error(w, "Error rendering template 001: "+err.Error(), statusCodeDefault)
		return
	}
	if err = tpl.ExecuteTemplate(w, "base.html", map[string]string{
		"title": title,
	}); err != nil {
		http.Error(w, "Error rendering template 002: "+err.Error(), statusCodeDefault)
		return
	}
}

func updateDataCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, r.Method+" Not authorized", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		renderTemplate(w, "Form cannot be parsed : "+err.Error())
		return
	}
	postValues := r.Form
	if postValues.Get("contentType") == "" || postValues.Get("contentValue") == "" {
		renderTemplate(w, "You have to fill all the fields")
		return
	}
	if postValues.Get("contentType") != "redirect" && postValues.Get("contentType") != "html" {
		renderTemplate(w, "Your content type is not valid")
		return
	}
	data := Data{
		DateAdd:      time.Now().UTC().Truncate(time.Second).Format("2006-01-02 15:04:05"),
		ContentValue: postValues.Get("contentValue"),
		ContentType:  postValues.Get("contentType"),
	}
	smtp, err := DB.Prepare("INSERT INTO data(date_add,content_value,content_type) VALUES(?,?,?)")
	if err != nil {
		renderTemplate(w, "Something wrong happened during prepare : "+err.Error())
		return
	}
	defer smtp.Close()
	_, err = smtp.Exec(data.DateAdd, data.ContentValue, data.ContentType)
	if err != nil {
		renderTemplate(w, "Something wrong happened during exec : "+err.Error())
		return
	}
	renderTemplate(w, "Resources has been successfully added", map[string]any{
		"code": http.StatusOK,
	})
}

func readData(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query("SELECT id,date_add,content_value,content_type FROM data ORDER BY id DESC LIMIT 1")
	if err != nil {
		http.Error(w, "Resource not found : "+err.Error(), http.StatusNotFound)
		return
	}
	if rows.Err() != nil {
		http.Error(w, "Row error : "+rows.Err(), http.StatusNotFound)
		return
	}
	var data Data
	for rows.Next() {
		if err := rows.Scan(&data.ID, &data.DateAdd, &data.ContentValue, &data.ContentType); err != nil {
			http.Error(w, "Error Scan : "+err.Error(), http.StatusNotFound)
			return
		}
	}
	if data.ID <= 0 {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}
	if data.ContentType == "redirect" {
		http.Redirect(w, r, data.ContentValue, http.StatusMovedPermanently)
		return
	}
	w.Header().Set("Content-type", "text/html")
	w.Write([]byte(data.ContentValue))
}
