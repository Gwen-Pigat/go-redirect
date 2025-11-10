package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
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

func main() {
	defer DB.Close()

	staticDir := "static"
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
	w.Write([]byte(`
	<html>
            <head><title>Create new resource</title></head>
            <body>
                <h1>New resource</h1>
				<a href="/">Return</a>
				<form action="/update/call" method="POST">
				<select required="required" name="contentType">
					<option value="redirect">Redirect</option>
					<option value="html">HTML</option>
				</select>
                <textarea name="contentValue" required style="min-height:150px"></textarea>
				<button type="submit">Submit</button>
				</form>
            </body>
        </html>`))
}

func renderHTML(w http.ResponseWriter, statusCode int, title string) {
	w.Header().Set("Content-type", "text/html")
	w.WriteHeader(statusCode)
	w.Write([]byte(`
	<html>
            <head><title>` + title + `</title></head>
            <body>
                <h1 style="text-align:center">` + title + `</h1>
				<a href="/update">Return to form</a>
            </body>
        </html>`))
}

func updateDataCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, r.Method+" Not authorized", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		renderHTML(w, http.StatusBadRequest, "Form cannot be parsed : "+err.Error())
		return
	}
	postValues := r.Form

	if postValues.Get("contentType") == "" || postValues.Get("contentValue") == "" {
		renderHTML(w, http.StatusBadRequest, "You have to fill all the fields")
		return
	}
	if postValues.Get("contentType") != "redirect" && postValues.Get("contentType") != "html" {
		renderHTML(w, http.StatusBadRequest, "Your content type is not valid")
		return
	}
	data := Data{
		DateAdd:      time.Now().UTC().Truncate(time.Second).Format("2006-01-02 15:04:05"),
		ContentValue: postValues.Get("contentValue"),
		ContentType:  postValues.Get("contentType"),
	}
	smtp, err := DB.Prepare("INSERT INTO data(date_add,content_value,content_type) VALUES(?,?,?)")
	if err != nil {
		renderHTML(w, http.StatusBadRequest, "Something wrong happened during prepare : "+err.Error())
		return
	}
	defer smtp.Close()
	_, err = smtp.Exec(data.DateAdd, data.ContentValue, data.ContentType)
	if err != nil {
		renderHTML(w, http.StatusBadRequest, "Something wrong happened during exec : "+err.Error())
		return
	}
	renderHTML(w, http.StatusOK, "Resources has been successfully added")
}

func readData(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query("SELECT id,date_add,content_value,content_type FROM data ORDER BY id DESC LIMIT 1")
	if err != nil {
		http.Error(w, "Resource not found", http.StatusNotFound)
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
