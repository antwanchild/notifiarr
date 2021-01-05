package apps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

// radarrHandlers is called once on startup to register the web API paths.
func (a *Apps) radarrHandlers() {
	a.HandleAPIpath(Radarr, "/add", radarrAddMovie, "POST")
	a.HandleAPIpath(Radarr, "/check/{tmdbid:[0-9]+}", radarrCheckMovie, "GET")
	a.HandleAPIpath(Radarr, "/get/{movieid:[0-9]+}", radarrGetMovie, "GET")
	a.HandleAPIpath(Radarr, "/qualityProfiles", radarrProfiles, "GET")
	a.HandleAPIpath(Radarr, "/rootFolder", radarrRootFolders, "GET")
	a.HandleAPIpath(Radarr, "/search/{query}", radarrSearchMovie, "GET")
	a.HandleAPIpath(Radarr, "/update", radarrUpdateMovie, "PUT")
}

// RadarrConfig represents the input data for a Radarr server.
type RadarrConfig struct {
	*starr.Config
	radarr *radarr.Radarr
}

func (r *RadarrConfig) setup(timeout time.Duration) {
	r.radarr = radarr.New(r.Config)
	if r.Timeout.Duration == 0 {
		r.Timeout.Duration = timeout
	}
}

func radarrAddMovie(r *http.Request) (int, interface{}) {
	var payload radarr.AddMovieInput
	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	} else if payload.TmdbID == 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("0: %w", ErrNoTMDB)
	}

	app := getRadarr(r)
	// Check for existing movie.
	m, err := app.GetMovie(payload.TmdbID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking movie: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, fmt.Errorf("%d: %w", payload.TmdbID, ErrExists)
	}

	if payload.Title == "" {
		// Title must exist, even if it's wrong.
		payload.Title = strconv.FormatInt(payload.TmdbID, 10)
	}

	if payload.MinimumAvailability == "" {
		payload.MinimumAvailability = "released"
	}

	// Add movie using fixed payload.
	movie, err := app.AddMovie(&payload)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding movie: %w", err)
	}

	return http.StatusCreated, movie
}

func radarrCheckMovie(r *http.Request) (int, interface{}) {
	tmdbID, _ := strconv.ParseInt(mux.Vars(r)["tmdbid"], 10, 64)
	// Check for existing movie.
	m, err := getRadarr(r).GetMovie(tmdbID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking movie: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, fmt.Errorf("%d: %w", tmdbID, ErrExists)
	}

	return http.StatusOK, http.StatusText(http.StatusNotFound)
}

func radarrGetMovie(r *http.Request) (int, interface{}) {
	movieID, _ := strconv.ParseInt(mux.Vars(r)["movieid"], 10, 64)

	movie, err := getRadarr(r).GetMovieByID(movieID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking movie: %w", err)
	}

	return http.StatusOK, movie
}

func radarrProfiles(r *http.Request) (int, interface{}) {
	// Get the profiles from radarr.
	profiles, err := getRadarr(r).GetQualityProfiles()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	// Format profile ID=>Name into a nice map.
	p := make(map[int64]string)
	for i := range profiles {
		p[profiles[i].ID] = profiles[i].Name
	}

	return http.StatusOK, p
}

func radarrRootFolders(r *http.Request) (int, interface{}) {
	// Get folder list from Radarr.
	folders, err := getRadarr(r).GetRootFolders()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting folders: %w", err)
	}

	// Format folder list into a nice path=>freesSpace map.
	p := make(map[string]int64)
	for i := range folders {
		p[folders[i].Path] = folders[i].FreeSpace
	}

	return http.StatusOK, p
}

func radarrSearchMovie(r *http.Request) (int, interface{}) {
	// Get all movies
	movies, err := getRadarr(r).GetMovie(0)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("getting movies: %w", err)
	}

	query := strings.TrimSpace(strings.ToLower(mux.Vars(r)["query"])) // in
	returnMovies := make([]map[string]interface{}, 0)                 // out

	for _, movie := range movies {
		if movieSearch(query, []string{movie.Title, movie.OriginalTitle}, movie.AlternateTitles) {
			returnMovies = append(returnMovies, map[string]interface{}{
				"id":                  movie.ID,
				"title":               movie.Title,
				"cinemas":             movie.InCinemas,
				"status":              movie.Status,
				"exists":              movie.HasFile,
				"added":               movie.Added,
				"year":                movie.Year,
				"path":                movie.Path,
				"tmdbId":              movie.TmdbID,
				"qualityProfileId":    movie.QualityProfileID,
				"monitored":           movie.Monitored,
				"minimumAvailability": movie.MinimumAvailability,
			})
		}
	}

	return http.StatusOK, returnMovies
}

func movieSearch(query string, titles []string, alts []*radarr.AlternativeTitle) bool {
	for _, t := range titles {
		if t != "" && strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}

	for _, t := range alts {
		if strings.Contains(strings.ToLower(t.Title), query) {
			return true
		}
	}

	return false
}

func radarrUpdateMovie(r *http.Request) (int, interface{}) {
	var movie radarr.Movie
	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&movie)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Check for existing movie.
	err = getRadarr(r).UpdateMovie(movie.ID, &movie)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating movie: %w", err)
	}

	return http.StatusOK, "radarr seems to have worked"
}
