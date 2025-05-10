//Date: 5-4-2025
// Built with Go and HTMX.
//Notes: the variables associated with "results" is part of the search function (the result of searching something)
//anything associated with the word answer is the actual response when everything is done
//List is the favorite list of the person

package main

import (
  "fmt"
  "net/http"
  "html/template"
  "log"
  "strconv"
  "strings"
  "os/exec"
  "sync"
  "time"
  _ "github.com/glebarez/go-sqlite"
  "database/sql"
  "encoding/json"
  "net/url"
)


type Anime struct {
  Name string
  Year int
  Rating float64
  Img string
}


func newAnime(name string, year int, rating float64, img string) *Anime{
  a := Anime{Name: name, Year: year, Rating: rating, Img: img}
  return &a
}


var userPicks = make(map[string][]*Anime)
var userPicksMutex sync.RWMutex


func openBrowser(url string) {
	var err error
	err = exec.Command("xdg-open", url).Start()
	if err != nil {
		log.Println("Error opening browser:", err)
	}
}


func getUserID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("user_id")
	if err != nil {
		id := strconv.FormatInt(time.Now().UnixNano(), 36)
		http.SetCookie(w, &http.Cookie{
			Name:  "user_id",
			Value: id,
			Path:  "/",
		})
		return id
	}
	return cookie.Value
}


func addToDatabase(db *sql.DB, addingAnime *Anime) {
  _, err := db.Exec(`INSERT INTO anime (name, year, rating, img) VALUES (?, ?, ?, ?)`, addingAnime.Name, addingAnime.Year, addingAnime.Rating, addingAnime.Img)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Printf("Duplicate anime '%s' not added to database.", addingAnime.Name)
			return // Don't treat this as a fatal error
		}
		log.Fatal("Error adding anime to database:", err)
	}
}



func setUpDatabase() *sql.DB {
	db, err := sql.Open("sqlite", "./anime.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create the table (including the 'img' column)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS anime (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT COLLATE NOCASE UNIQUE,
			year INTEGER,
			rating REAL,
			img TEXT
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
	// Check if database is empty
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM anime").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	// Insert default anime if empty
	if count == 0 {
		animes := []Anime{
			{"Fullmetal Alchemist: Brotherhood", 2012, 10.0, "https://cdn.myanimelist.net/images/anime/1223/96541.jpg"},
			{"Fullmetal Alchemist", 2004, 8.0, "https://cdn.myanimelist.net/images/anime/1223/96541.jpg"},
		}
		for _, a := range animes {
			_, err := db.Exec(
				`INSERT INTO anime (name, year, rating, img) VALUES (?, ?, ?, ?)`,
				a.Name, a.Year, a.Rating, a.Img,
			)
			if err != nil {
				log.Fatal(err)
			}
		}
		fmt.Println("The anime has been added to the database as well!")
	}
	return db
}


func searchTheDatabase(db *sql.DB, query string) []*Anime {
  var results []*Anime
  rows, err := db.Query("SELECT name, year, rating, img FROM anime WHERE name LIKE ?", "%"+query+"%")
  if err != nil {
    log.Fatal(err)
  }
  defer rows.Close()
  for rows.Next() {
    var name string
    var year int
    var rating float64
    var img string
    err := rows.Scan(&name, &year, &rating, &img)
    if err != nil {
      log.Println("Error scanning row:", err)
      continue
    }
    results = append(results, newAnime(name, year, rating, img))
    if len(results) > 2 {
      break
    }
  }
  return results
}



type JikanAnime struct {
	Data []struct {
		Title string  `json:"title"`
		Year  int     `json:"year"`   // Optional field (may be 0)
		Score float64 `json:"score"`  // MAL rating

		Images struct {
			JPG struct {
				Img string `json:"image_url"`
			} `json:"jpg"`
		} `json:"images"`
	} `json:"data"`
}



// fetchFromJikan queries the Jikan API for anime data based on a search term
func fetchFromJikan(query string) (*Anime, error) {
	// Encode the search query using url.QueryEscape()
	encodedQuery := url.QueryEscape(query)
  time.Sleep(1 * time.Second)
	// Construct the API URL using the encoded search query, limit to 1 result
	apiURL := fmt.Sprintf("https://api.jikan.moe/v4/anime?q=%s&limit=1", encodedQuery)
	fmt.Println("Jikan API URL:", apiURL) // For debugging
	// Make an HTTP GET request to the Jikan API
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err // If the request fails, return the error
	}
	defer resp.Body.Close() // Always close the response body when done
  if resp.StatusCode != http.StatusOK {
    log.Printf("Jikan API returned status code: %d for query '%s'", resp.StatusCode, query)
    return nil, fmt.Errorf("Jikan API returned an error: %s", resp.Status)
  }
	// Create a variable to hold the parsed JSON data
	var jikanResult JikanAnime
	// Decode the JSON response into our JikanAnime struct
	err = json.NewDecoder(resp.Body).Decode(&jikanResult)
	if err != nil {
		return nil, err // If decoding fails, return the error
	}
	// If no anime was found, return an error
	if len(jikanResult.Data) == 0 {
    log.Printf("Jikan API returned no results for '%s': %+v", query, jikanResult)
    return nil, fmt.Errorf("no anime found for %s", query)
	}
	// Extract the first anime from the results
	jikanSingleData := jikanResult.Data[0]
	// Fallback for year if it's missing (sometimes Jikan doesn't include it)
	year := jikanSingleData.Year
	if year == 0 {
		year = 2016
	}
	// Create your local Anime struct using the fetched database
  anime := newAnime(jikanSingleData.Title, year, jikanSingleData.Score, jikanSingleData.Images.JPG.Img)
	return anime, nil
}

func main() {
    //yes, this is what it is
    db := setUpDatabase()
    defer db.Close()
    fmt.Println("Hello! Going to start the web server!")
    
    //now this is the function keep in mind that h1 means handler #1 NOT heading 1 (HTML)
    h1 := func (w http.ResponseWriter, r *http.Request) {
      tmp1 := template.Must(template.ParseFiles("index.html"))
      tmp1.Execute(w, nil)
    }


    //You need this in order to use other things like pictures and css in your website
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
    //since this is "/" basicly when you go to the homepage it tells the person Hey! go run h1 when visitng the homepage
    http.HandleFunc("/", h1)


    http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		  // Get the anime from the query string
		  animeNamePick := r.FormValue("anime")
      yearPickStr := r.FormValue("year")
      ratingPickStr := r.FormValue("rating")
      imgPick := r.FormValue("img")
      yearPick, err2 := strconv.Atoi(yearPickStr)
      ratingPick, err1 := strconv.ParseFloat(ratingPickStr, 64)
      if err1 != nil || err2 != nil {
			  http.Error(w, "Invalid input.", http.StatusBadRequest)
			  return
		  }
      //Need to make everyone's picks seperate and unique
      userID := getUserID(w, r)
      animePick := newAnime(animeNamePick, yearPick, ratingPick, imgPick)
      // 🔒 Safe write
      userPicksMutex.Lock()
      userPicks[userID] = append(userPicks[userID], animePick)
      userPicksMutex.Unlock()
      // 🔒 Safe read
      userPicksMutex.RLock()
      userList := userPicks[userID]
      userPicksMutex.RUnlock()
      w.Header().Set("Content-Type", "text/html")
      i := 1
      w.Write([]byte("<hr>These are is the list of anime that you picked: <br> <table><tr>"))
      for _, theAnime := range userList {
        escapedImg := template.HTMLEscapeString(theAnime.Img)
        html := fmt.Sprintf(`<td><img src="%s" height="150"> </td>`, escapedImg)
        w.Write([]byte(html))
        i++
        if i > 4 {
            break
        }
      }
      w.Write([]byte("</tr><tr>"))
      i = 1
      for _, theAnime := range userList {
        escapedName := template.HTMLEscapeString(theAnime.Name)
        w.Write([]byte("<td>" + escapedName + "</td>"))
        i++
        if i > 4 {
            break
        }
      }
      w.Write([]byte("</td></tr></table>"))
	  })


  http.HandleFunc("/addSubmit", func(w http.ResponseWriter, r *http.Request) {
    userID := getUserID(w, r)
    // 🔒 Safe read of the user's list
    userPicksMutex.RLock()
    userList := userPicks[userID]
    userPicksMutex.RUnlock()
    if len(userList) >= 4 {
      w.Header().Set("Content-Type", "text/html")
      w.Write([]byte(`<button hx-post="/answer" hx-target="#answer">Submit!</button>`))
    }
  })


  http.HandleFunc("/answer", func(w http.ResponseWriter, r *http.Request) {
    userID := getUserID(w, r)
    // 🔒 Safe read of the user's list
    userPicksMutex.RLock()
    userList := userPicks[userID]
    userPicksMutex.RUnlock()
    score := 0.0
    i := 1
    for _, theAnime := range userList {
      if theAnime.Rating == 0 {
        score = score + 7.0
      } else {
        score = score + theAnime.Rating
      }
      i++
      if i > 4 {
        break
      }
    }
    score = score / 4
	  w.Header().Set("Content-Type", "text/html")
    if score > 7.8 {
    w.Write([]byte("<br><strong>Yeah, it seems like you got good taste!</strong><br>Whenever you get onto an aruguement on the internet know that your opinon is obviously the correct one!"))
    } else if score > 7.0 && score < 7.8 {
      w.Write([]byte("<strong> Your taste in anime is... alright.</strong><br>You're opinion might not always be right but you're probably fun to talk to at anime conventions."))
    } else {
      w.Write([]byte("<strong>You're taste is trash!</strong><br>You should spend more time laying down and reconsider all your life choices."))
    }
    w.Write([]byte("<br><strong>Your score: " + strconv.FormatFloat(score, 'f', 1, 64) + " </strong>"))
    // Reset the user's list to an empty slice
    userPicksMutex.Lock() // Acquire write lock before modifying the map
    userPicks[userID] = []*Anime{}
    userPicksMutex.Unlock() // Release write lock
  })



http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
    // Get the value the user typed in the search box. This matches the 'name="anime1"' from your HTML input.
    query := r.URL.Query().Get("anime1")
    // Tell the browser that we're sending back HTML (not plain txt)
    w.Header().Set("Content-Type", "text/html")
    // If the search box is empty, return nothing
    if query == "" {
        w.Write([]byte("")) // nothing is written to the page
    } else {
        results := searchTheDatabase(db, query)
        if len(results) < 3 {
          //check jikan
          animeFromJikan, err := fetchFromJikan(query)
          if err != nil {
            fmt.Println("Error fetching anime: ", err)
            return
          }
          isItAlreadyThere := false
          for _, result := range results {
            if animeFromJikan.Name == result.Name && animeFromJikan.Year == animeFromJikan.Year{
              isItAlreadyThere = true
            }
          }
          if isItAlreadyThere == false {
            addToDatabase(db, animeFromJikan)
            results = append(results, animeFromJikan)
          }
        }
        i := 1
        //now actually displaying the results
        for _, result := range results {
            // Escape potentially dangerous user-controlled content
            escapedName := template.HTMLEscapeString(result.Name)
            escapedImg := template.HTMLEscapeString(result.Img)

            // fmt.Sprintf formats the string with the anime title in <p> tags
            html := fmt.Sprintf(
              `<p>%s (%d) <button hx-post="/add" hx-vals='{"anime":"%s","year":%d,"rating":%.1f, "img":"%s"}' hx-target="#picks">Add</button></p>`,
              escapedName, result.Year, escapedName, result.Year, result.Rating, escapedImg,
            )
            w.Write([]byte(html))
            i++
            if i > 3 {
              break
            }
        }
    }
})


    openBrowser("http://localhost:8000")
    //log.fatal will recond something if it failed to make a webserver. ListenAndServe creates that server
    log.Fatal(http.ListenAndServe(":8000", nil)) 
  }
