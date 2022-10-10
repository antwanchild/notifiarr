//nolint:dupl
package apps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/gorilla/mux"
	"golift.io/starr"
	"golift.io/starr/debuglog"
	"golift.io/starr/sonarr"
)

// sonarrHandlers is called once on startup to register the web API paths.
func (a *Apps) sonarrHandlers() {
	a.HandleAPIpath(starr.Sonarr, "/add", sonarrAddSeries, "POST")
	a.HandleAPIpath(starr.Sonarr, "/check/{tvdbid:[0-9]+}", sonarrCheckSeries, "GET")
	a.HandleAPIpath(starr.Sonarr, "/get/{seriesid:[0-9]+}", sonarrGetSeries, "GET")
	a.HandleAPIpath(starr.Sonarr, "/getEpisodes/{seriesid:[0-9]+}", sonarrGetEpisodes, "GET")
	a.HandleAPIpath(starr.Sonarr, "/unmonitor/{episodeid:[0-9]+}", sonarrUnmonitorEpisode, "GET")
	a.HandleAPIpath(starr.Sonarr, "/languageProfiles", sonarrLangProfiles, "GET")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfiles", sonarrGetQualityProfiles, "GET")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfile", sonarrGetQualityProfile, "GET")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfile", sonarrAddQualityProfile, "POST")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfile/{profileID:[0-9]+}", sonarrUpdateQualityProfile, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfile/{profileID:[0-9]+}", sonarrDeleteQualityProfile, "DELETE")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfiles/all", sonarrDeleteAllQualityProfiles, "DELETE")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfiles", sonarrGetReleaseProfiles, "GET")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfile", sonarrAddReleaseProfile, "POST")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfile/{profileID:[0-9]+}", sonarrUpdateReleaseProfile, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfile/{profileID:[0-9]+}", sonarrDeleteReleaseProfile, "DELETE")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfiles/all", sonarrDeleteAllReleaseProfiles, "DELETE")
	a.HandleAPIpath(starr.Sonarr, "/customformats", sonarrGetCustomFormats, "GET")
	a.HandleAPIpath(starr.Sonarr, "/customformats", sonarrAddCustomFormat, "POST")
	a.HandleAPIpath(starr.Sonarr, "/customformats/{cfid:[0-9]+}", sonarrUpdateCustomFormat, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/customformats/{cfid:[0-9]+}", sonarrDeleteCustomFormat, "DELETE")
	a.HandleAPIpath(starr.Sonarr, "/customformats/all", sonarrDeleteAllCustomFormats, "DELETE")
	a.HandleAPIpath(starr.Sonarr, "/qualitydefinitions", sonarrGetQualityDefinitions, "GET")
	a.HandleAPIpath(starr.Sonarr, "/qualitydefinition", sonarrUpdateQualityDefinition, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/rootFolder", sonarrRootFolders, "GET")
	a.HandleAPIpath(starr.Sonarr, "/search/{query}", sonarrSearchSeries, "GET")
	a.HandleAPIpath(starr.Sonarr, "/tag", sonarrGetTags, "GET")
	a.HandleAPIpath(starr.Sonarr, "/tag/{tid:[0-9]+}/{label}", sonarrUpdateTag, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/tag/{label}", sonarrSetTag, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/update", sonarrUpdateSeries, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/seasonPass", sonarrSeasonPass, "POST")
	a.HandleAPIpath(starr.Sonarr, "/command/{commandid:[0-9]+}", sonarrStatusCommand, "GET")
	a.HandleAPIpath(starr.Sonarr, "/command", sonarrTriggerCommand, "POST")
	a.HandleAPIpath(starr.Sonarr, "/command/search/{seriesid:[0-9]+}", sonarrTriggerSearchSeries, "GET")
}

// SonarrConfig represents the input data for a Sonarr server.
type SonarrConfig struct {
	*sonarr.Sonarr `toml:"-" xml:"-" json:"-"`
	extraConfig
	*starr.Config
	errorf func(string, ...interface{}) `toml:"-" xml:"-" json:"-"`
}

func getSonarr(r *http.Request) *sonarr.Sonarr {
	app, _ := r.Context().Value(starr.Sonarr).(*SonarrConfig)
	return app.Sonarr
}

// Enabled returns true if the Sonarr instance is enabled and usable.
func (s *SonarrConfig) Enabled() bool {
	return s != nil && s.Config != nil && s.URL != "" && s.APIKey != "" && s.Timeout.Duration >= 0
}

func (a *Apps) setupSonarr() error {
	for idx, app := range a.Sonarr {
		if app.Config == nil || app.Config.URL == "" {
			return fmt.Errorf("%w: missing url: Sonarr config %d", ErrInvalidApp, idx+1)
		} else if !strings.HasPrefix(app.Config.URL, "http://") && !strings.HasPrefix(app.Config.URL, "https://") {
			return fmt.Errorf("%w: URL must begin with http:// or https://: Sonarr config %d", ErrInvalidApp, idx+1)
		}

		if a.Logger.DebugEnabled() {
			app.Config.Client = starr.ClientWithDebug(app.Timeout.Duration, app.ValidSSL, debuglog.Config{
				MaxBody: a.MaxBody,
				Debugf:  a.Debugf,
				Caller:  metricMakerCallback(string(starr.Sonarr)),
			})
		} else {
			app.Config.Client = starr.Client(app.Timeout.Duration, app.ValidSSL)
			app.Config.Client.Transport = NewMetricsRoundTripper(starr.Sonarr.String(), nil)
		}

		app.errorf = a.Errorf
		app.URL = strings.TrimRight(app.URL, "/")
		app.Sonarr = sonarr.New(app.Config)
	}

	return nil
}

// @Description  Adds a new Series to Sonarr.
// @Summary      Add Sonarr Series
// @Tags         sonarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        POST body sonarr.AddSeriesInput true "new item content"
// @Accept       json
// @Success      201  {object} apps.Respond.apiResponse{message=sonarr.Series} "series content"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "bad json payload"
// @Failure      409  {object} apps.Respond.apiResponse{message=string} "item alrady exists"
// @Failure      422  {object} apps.Respond.apiResponse{message=string} "no item ID provided"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error during check"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error during add"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/sonarr/{instance}/add [post]
// @Security     ApiKeyAuth
func sonarrAddSeries(req *http.Request) (int, interface{}) {
	var payload sonarr.AddSeriesInput
	// Extract payload and check for TVDB ID.
	err := json.NewDecoder(req.Body).Decode(&payload)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	} else if payload.TvdbID == 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("0: %w", ErrNoTVDB)
	}

	// Check for existing series.
	m, err := getSonarr(req).GetSeriesContext(req.Context(), payload.TvdbID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, sonarrData(m[0])
	}

	series, err := getSonarr(req).AddSeriesContext(req.Context(), &payload)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding series: %w", err)
	}

	return http.StatusCreated, series
}

func sonarrData(series *sonarr.Series) map[string]interface{} {
	hasFile := false
	if series.Statistics != nil {
		hasFile = series.Statistics.SizeOnDisk > 0
	}

	return map[string]interface{}{
		"id":        series.ID,
		"hasFile":   hasFile,
		"monitored": series.Monitored,
		"tags":      series.Tags,
	}
}

func sonarrCheckSeries(req *http.Request) (int, interface{}) {
	tvdbid, _ := strconv.ParseInt(mux.Vars(req)["tvdbid"], mnd.Base10, mnd.Bits64)
	// Check for existing series.
	m, err := getSonarr(req).GetSeriesContext(req.Context(), tvdbid)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, sonarrData(m[0])
	}

	return http.StatusOK, http.StatusText(http.StatusNotFound)
}

func sonarrGetSeries(req *http.Request) (int, interface{}) {
	seriesID, _ := strconv.ParseInt(mux.Vars(req)["seriesid"], mnd.Base10, mnd.Bits64)

	series, err := getSonarr(req).GetSeriesByIDContext(req.Context(), seriesID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	}

	return http.StatusOK, series
}

func sonarrGetEpisodes(req *http.Request) (int, interface{}) {
	seriesID, _ := strconv.ParseInt(mux.Vars(req)["seriesid"], mnd.Base10, mnd.Bits64)

	episodes, err := getSonarr(req).GetSeriesEpisodesContext(req.Context(), seriesID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	}

	return http.StatusOK, episodes
}

func sonarrUnmonitorEpisode(req *http.Request) (int, interface{}) {
	episodeID, _ := strconv.ParseInt(mux.Vars(req)["episodeid"], mnd.Base10, mnd.Bits64)

	episodes, err := getSonarr(req).MonitorEpisodeContext(req.Context(), []int64{episodeID}, false)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	} else if len(episodes) != 1 {
		return http.StatusServiceUnavailable, fmt.Errorf("%w (%d): %v", ErrWrongCount, len(episodes), episodes)
	}

	return http.StatusOK, episodes[0]
}

func sonarrTriggerSearchSeries(req *http.Request) (int, interface{}) {
	seriesID, _ := strconv.ParseInt(mux.Vars(req)["seriesid"], mnd.Base10, mnd.Bits64)

	output, err := getSonarr(req).SendCommandContext(req.Context(), &sonarr.CommandRequest{
		Name:     "SeriesSearch",
		SeriesID: seriesID,
	})
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("triggering series search: %w", err)
	}

	return http.StatusOK, output.Status
}

func sonarrTriggerCommand(req *http.Request) (int, interface{}) {
	var command sonarr.CommandRequest

	err := json.NewDecoder(req.Body).Decode(&command)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding command payload: %w", err)
	}

	output, err := getSonarr(req).SendCommandContext(req.Context(), &command)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("triggering command '%s' on series %d: %w", command.Name, command.SeriesID, err)
	}

	return http.StatusOK, output
}

func sonarrStatusCommand(req *http.Request) (int, interface{}) {
	commandID, _ := strconv.ParseInt(mux.Vars(req)["commandid"], mnd.Base10, mnd.Bits64)

	output, err := getSonarr(req).GetCommandStatusContext(req.Context(), commandID)
	if err != nil {
		return http.StatusServiceUnavailable,
			fmt.Errorf("getting command status for ID %d: %w", commandID, err)
	}

	return http.StatusOK, output
}

func sonarrLangProfiles(req *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(req).GetLanguageProfilesContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting language profiles: %w", err)
	}

	// Format profile ID=>Name into a nice map.
	p := make(map[int64]string)
	for i := range profiles {
		p[profiles[i].ID] = profiles[i].Name
	}

	return http.StatusOK, p
}

func sonarrGetQualityProfile(req *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(req).GetQualityProfilesContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	return http.StatusOK, profiles
}

func sonarrGetQualityProfiles(req *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(req).GetQualityProfilesContext(req.Context())
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

func sonarrAddQualityProfile(req *http.Request) (int, interface{}) {
	var profile sonarr.QualityProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Get the profiles from sonarr.
	id, err := getSonarr(req).AddQualityProfileContext(req.Context(), &profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding profile: %w", err)
	}

	return http.StatusOK, id
}

func sonarrUpdateQualityProfile(req *http.Request) (int, interface{}) {
	var profile sonarr.QualityProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	profile.ID, _ = strconv.ParseInt(mux.Vars(req)["profileID"], mnd.Base10, mnd.Bits64)
	if profile.ID == 0 {
		return http.StatusBadRequest, ErrNonZeroID
	}

	// Get the profiles from sonarr.
	_, err = getSonarr(req).UpdateQualityProfileContext(req.Context(), &profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating profile: %w", err)
	}

	return http.StatusOK, "OK"
}

func sonarrDeleteQualityProfile(req *http.Request) (int, interface{}) {
	profileID, _ := strconv.Atoi(mux.Vars(req)["profileID"])
	if profileID == 0 {
		return http.StatusBadRequest, ErrNonZeroID
	}

	// Delete the profile from sonarr.
	err := getSonarr(req).DeleteQualityProfileContext(req.Context(), profileID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("deleting profile: %w", err)
	}

	return http.StatusOK, "OK"
}

func sonarrDeleteAllQualityProfiles(req *http.Request) (int, interface{}) {
	// Get all the profiles from sonarr.
	profiles, err := getSonarr(req).GetQualityProfilesContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	var (
		deleted int
		errs    []string
	)

	// Delete each profile from sonarr.
	for _, profile := range profiles {
		if err := getSonarr(req).DeleteQualityProfileContext(req.Context(), int(profile.ID)); err != nil {
			errs = append(errs, err.Error())
			continue
		}

		deleted++
	}

	return http.StatusOK, map[string]any{
		"found":   len(profiles),
		"deleted": deleted,
		"errors":  errs,
	}
}

func sonarrGetReleaseProfiles(req *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(req).GetReleaseProfilesContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	return http.StatusOK, profiles
}

func sonarrAddReleaseProfile(req *http.Request) (int, interface{}) {
	var profile sonarr.ReleaseProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Get the profiles from sonarr.
	id, err := getSonarr(req).AddReleaseProfileContext(req.Context(), &profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding profile: %w", err)
	}

	return http.StatusOK, id
}

func sonarrUpdateReleaseProfile(req *http.Request) (int, interface{}) {
	var profile sonarr.ReleaseProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	profile.ID, _ = strconv.ParseInt(mux.Vars(req)["profileID"], mnd.Base10, mnd.Bits64)
	if profile.ID == 0 {
		return http.StatusBadRequest, ErrNonZeroID
	}

	// Get the profiles from sonarr.
	_, err = getSonarr(req).UpdateReleaseProfileContext(req.Context(), &profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating profile: %w", err)
	}

	return http.StatusOK, "OK"
}

func sonarrDeleteReleaseProfile(req *http.Request) (int, interface{}) {
	profileID, _ := strconv.Atoi(mux.Vars(req)["profileID"])
	if profileID == 0 {
		return http.StatusBadRequest, ErrNonZeroID
	}

	// Delete the profile from sonarr.
	err := getSonarr(req).DeleteReleaseProfileContext(req.Context(), profileID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("deleting profile: %w", err)
	}

	return http.StatusOK, "OK"
}

func sonarrDeleteAllReleaseProfiles(req *http.Request) (int, interface{}) {
	profiles, err := getSonarr(req).GetReleaseProfilesContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	var (
		deleted int
		errs    []string
	)

	for _, profile := range profiles {
		// Delete the profile from sonarr.
		err := getSonarr(req).DeleteReleaseProfileContext(req.Context(), int(profile.ID))
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		deleted++
	}

	return http.StatusOK, map[string]any{
		"found":   len(profiles),
		"deleted": deleted,
		"errors":  errs,
	}
}

func sonarrRootFolders(req *http.Request) (int, interface{}) {
	// Get folder list from Sonarr.
	folders, err := getSonarr(req).GetRootFoldersContext(req.Context())
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

func sonarrSearchSeries(req *http.Request) (int, interface{}) {
	// Get all movies
	series, err := getSonarr(req).GetAllSeriesContext(req.Context())
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("getting series: %w", err)
	}

	query := strings.TrimSpace(mux.Vars(req)["query"]) // in
	resp := make([]map[string]interface{}, 0)          // out

	for _, item := range series {
		if seriesSearch(query, item.Title, item.AlternateTitles) {
			resp = append(resp, map[string]interface{}{
				"id":                item.ID,
				"title":             item.Title,
				"first":             item.FirstAired,
				"next":              item.NextAiring,
				"prev":              item.PreviousAiring,
				"added":             item.Added,
				"status":            item.Status,
				"path":              item.Path,
				"tvdbId":            item.TvdbID,
				"monitored":         item.Monitored,
				"qualityProfileId":  item.QualityProfileID,
				"seasonFolder":      item.SeasonFolder,
				"seriesType":        item.SeriesType,
				"languageProfileId": item.LanguageProfileID,
				"seasons":           item.Seasons,
				"exists":            item.Statistics != nil && item.Statistics.SizeOnDisk > 0,
			})
		}
	}

	return http.StatusOK, resp
}

func seriesSearch(query, title string, alts []*sonarr.AlternateTitle) bool {
	if strings.Contains(strings.ToLower(title), strings.ToLower(query)) {
		return true
	}

	for _, t := range alts {
		if strings.Contains(strings.ToLower(t.Title), strings.ToLower(query)) {
			return true
		}
	}

	return false
}

func sonarrGetTags(req *http.Request) (int, interface{}) {
	tags, err := getSonarr(req).GetTagsContext(req.Context())
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("getting tags: %w", err)
	}

	return http.StatusOK, tags
}

func sonarrUpdateTag(req *http.Request) (int, interface{}) {
	id, _ := strconv.Atoi(mux.Vars(req)["tid"])

	tag, err := getSonarr(req).UpdateTagContext(req.Context(), &starr.Tag{ID: id, Label: mux.Vars(req)["label"]})
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating tag: %w", err)
	}

	return http.StatusOK, tag.ID
}

func sonarrSetTag(req *http.Request) (int, interface{}) {
	tag, err := getSonarr(req).AddTagContext(req.Context(), &starr.Tag{Label: mux.Vars(req)["label"]})
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("setting tag: %w", err)
	}

	return http.StatusOK, tag.ID
}

func sonarrUpdateSeries(req *http.Request) (int, interface{}) {
	var series sonarr.AddSeriesInput

	err := json.NewDecoder(req.Body).Decode(&series)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	_, err = getSonarr(req).UpdateSeriesContext(req.Context(), &series)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating series: %w", err)
	}

	return http.StatusOK, "sonarr seems to have worked"
}

func sonarrSeasonPass(req *http.Request) (int, interface{}) {
	var seasonPass sonarr.SeasonPass

	err := json.NewDecoder(req.Body).Decode(&seasonPass)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	err = getSonarr(req).UpdateSeasonPassContext(req.Context(), &seasonPass)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating seasonPass: %w", err)
	}

	return http.StatusOK, "ok"
}

func sonarrAddCustomFormat(req *http.Request) (int, interface{}) {
	var cusform sonarr.CustomFormat

	err := json.NewDecoder(req.Body).Decode(&cusform)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	resp, err := getSonarr(req).AddCustomFormatContext(req.Context(), &cusform)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding custom format: %w", err)
	}

	return http.StatusOK, resp
}

func sonarrGetCustomFormats(req *http.Request) (int, interface{}) {
	cusform, err := getSonarr(req).GetCustomFormatsContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting custom formats: %w", err)
	}

	return http.StatusOK, cusform
}

func sonarrUpdateCustomFormat(req *http.Request) (int, interface{}) {
	var cusform sonarr.CustomFormat
	if err := json.NewDecoder(req.Body).Decode(&cusform); err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	cfID, _ := strconv.Atoi(mux.Vars(req)["cfid"])

	output, err := getSonarr(req).UpdateCustomFormatContext(req.Context(), &cusform, cfID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating custom format: %w", err)
	}

	return http.StatusOK, output
}

func sonarrDeleteCustomFormat(req *http.Request) (int, interface{}) {
	cfID, _ := strconv.Atoi(mux.Vars(req)["cfid"])

	err := getSonarr(req).DeleteCustomFormatContext(req.Context(), cfID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("deleting custom format: %w", err)
	}

	return http.StatusOK, "OK"
}

func sonarrDeleteAllCustomFormats(req *http.Request) (int, interface{}) {
	formats, err := getSonarr(req).GetCustomFormatsContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting custom formats: %w", err)
	}

	var (
		deleted int
		errs    []string
	)

	for _, format := range formats {
		err := getSonarr(req).DeleteCustomFormatContext(req.Context(), format.ID)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		deleted++
	}

	return http.StatusOK, map[string]any{
		"found":   len(formats),
		"deleted": deleted,
		"errors":  errs,
	}
}

func sonarrGetQualityDefinitions(req *http.Request) (int, interface{}) {
	output, err := getSonarr(req).GetQualityDefinitionsContext(req.Context())
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting quality definitions: %w", err)
	}

	return http.StatusOK, output
}

func sonarrUpdateQualityDefinition(req *http.Request) (int, interface{}) {
	var input []*sonarr.QualityDefinition
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	output, err := getSonarr(req).UpdateQualityDefinitionsContext(req.Context(), input)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating quality definition: %w", err)
	}

	return http.StatusOK, output
}
