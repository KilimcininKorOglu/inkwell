package dashboard

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"inkwell/internal/crypto"
	"inkwell/internal/models"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	db            *gorm.DB
	tmpl          *template.Template
	encryptionKey string
}

// NewRouter creates the Chi router with all dashboard routes.
func NewRouter(db *gorm.DB, templateDir, staticDir, adminUser, adminPassword, encryptionKey string) (chi.Router, error) {
	funcMap := template.FuncMap{
		"toJSON": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"eq": func(a, b uint) bool {
			return a == b
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(filepath.Join(templateDir, "**", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse partials: %w", err)
	}
	tmpl, err = tmpl.ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse layout: %w", err)
	}

	tmpl, err = tmpl.ParseGlob(filepath.Join(templateDir, "components", "*.html"))
	if err != nil {
		log.Printf("Note: no component templates found: %v", err)
	}

	h := &Handler{db: db, tmpl: tmpl, encryptionKey: encryptionKey}

	r := chi.NewRouter()

	// Static files (no auth required)
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir)))
	r.Handle("/static/*", fs)

	// Protected routes
	r.Group(func(r chi.Router) {
		if adminUser != "" && adminPassword != "" {
			r.Use(basicAuth(adminUser, adminPassword))
		}

		// Dashboard routes
		r.Get("/", h.handleIndex)
		r.Get("/dashboard/content", h.handleContent)
		r.Get("/dashboard/detail/{dbid}", h.handleDetail)

		// Domain management routes
		r.Get("/domains", h.handleDomainsList)
		r.Get("/domains/new", h.handleDomainNew)
		r.Post("/domains", h.handleDomainCreate)
		r.Get("/domains/{id}/edit", h.handleDomainEdit)
		r.Post("/domains/{id}", h.handleDomainUpdate)
		r.Post("/domains/{id}/delete", h.handleDomainDelete)
		r.Post("/domains/{id}/toggle", h.handleDomainToggle)
	})

	return r, nil
}

// --- Dashboard Handlers ---

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	pageData := h.buildPageData(r)
	if err := h.tmpl.ExecuteTemplate(w, "layout.html", pageData); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleContent(w http.ResponseWriter, r *http.Request) {
	pageData := h.buildPageData(r)
	if err := h.tmpl.ExecuteTemplate(w, "content", pageData); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleDetail(w http.ResponseWriter, r *http.Request) {
	dbidStr := chi.URLParam(r, "dbid")
	dbid, err := strconv.ParseUint(dbidStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid report ID", http.StatusBadRequest)
		return
	}

	detail, err := FetchReportDetail(h.db, uint(dbid))
	if err != nil {
		log.Printf("Error fetching report detail: %v", err)
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "detail", detail); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// --- Domain Management Handlers ---

func (h *Handler) handleDomainsList(w http.ResponseWriter, r *http.Request) {
	domains, err := FetchAllDomains(h.db)
	if err != nil {
		log.Printf("Error fetching domains: %v", err)
	}

	data := DomainsPageData{
		Domains: domains,
		Message: r.URL.Query().Get("msg"),
	}

	if r.Header.Get("HX-Request") == "true" {
		h.tmpl.ExecuteTemplate(w, "domainsList", data)
	} else {
		h.tmpl.ExecuteTemplate(w, "domainsLayout", data)
	}
}

func (h *Handler) handleDomainNew(w http.ResponseWriter, r *http.Request) {
	data := DomainFormData{
		Domain: DomainFormValues{
			IMAPPort:   993,
			IMAPFolder: "INBOX",
			Enabled:    true,
		},
		IsEdit: false,
	}

	if r.Header.Get("HX-Request") == "true" {
		h.tmpl.ExecuteTemplate(w, "domainForm", data)
	} else {
		h.tmpl.ExecuteTemplate(w, "domainFormLayout", data)
	}
}

func (h *Handler) handleDomainCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	port, _ := strconv.Atoi(r.FormValue("imap_port"))
	if port == 0 {
		port = 993
	}

	// Encrypt password
	password := r.FormValue("imap_password")
	if password != "" && h.encryptionKey != "" {
		encrypted, err := crypto.Encrypt(password, h.encryptionKey)
		if err != nil {
			log.Printf("Error encrypting password: %v", err)
			h.renderDomainFormError(w, r, "Failed to encrypt password", false, r.Form)
			return
		}
		password = encrypted
	}

	domain := models.Domain{
		Name:              r.FormValue("name"),
		IMAPServer:        r.FormValue("imap_server"),
		IMAPPort:          port,
		IMAPUser:          r.FormValue("imap_user"),
		IMAPPassword:      password,
		IMAPFolder:        r.FormValue("imap_folder"),
		IMAPMoveFolder:    r.FormValue("imap_move_folder"),
		IMAPMoveFolderErr: r.FormValue("imap_move_folder_err"),
		Enabled:           r.FormValue("enabled") == "on",
	}

	if err := CreateDomain(h.db, &domain); err != nil {
		log.Printf("Error creating domain: %v", err)
		h.renderDomainFormError(w, r, "Failed to create domain: "+err.Error(), false, r.Form)
		return
	}

	http.Redirect(w, r, "/domains?msg=Domain+created+successfully", http.StatusSeeOther)
}

func (h *Handler) handleDomainEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	domain, err := FetchDomainByID(h.db, id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	data := DomainFormData{
		Domain: DomainFormValues{
			ID:                domain.ID,
			Name:              domain.Name,
			IMAPServer:        domain.IMAPServer,
			IMAPPort:          domain.IMAPPort,
			IMAPUser:          domain.IMAPUser,
			IMAPFolder:        domain.IMAPFolder,
			IMAPMoveFolder:    domain.IMAPMoveFolder,
			IMAPMoveFolderErr: domain.IMAPMoveFolderErr,
			Enabled:           domain.Enabled,
		},
		IsEdit: true,
	}

	if r.Header.Get("HX-Request") == "true" {
		h.tmpl.ExecuteTemplate(w, "domainForm", data)
	} else {
		h.tmpl.ExecuteTemplate(w, "domainFormLayout", data)
	}
}

func (h *Handler) handleDomainUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	existing, err := FetchDomainByID(h.db, id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	port, _ := strconv.Atoi(r.FormValue("imap_port"))
	if port == 0 {
		port = 993
	}

	// Keep existing encrypted password if field is blank
	password := existing.IMAPPassword
	if newPass := r.FormValue("imap_password"); newPass != "" && h.encryptionKey != "" {
		encrypted, err := crypto.Encrypt(newPass, h.encryptionKey)
		if err != nil {
			log.Printf("Error encrypting password: %v", err)
			h.renderDomainFormError(w, r, "Failed to encrypt password", true, r.Form)
			return
		}
		password = encrypted
	}

	existing.Name = r.FormValue("name")
	existing.IMAPServer = r.FormValue("imap_server")
	existing.IMAPPort = port
	existing.IMAPUser = r.FormValue("imap_user")
	existing.IMAPPassword = password
	existing.IMAPFolder = r.FormValue("imap_folder")
	existing.IMAPMoveFolder = r.FormValue("imap_move_folder")
	existing.IMAPMoveFolderErr = r.FormValue("imap_move_folder_err")
	existing.Enabled = r.FormValue("enabled") == "on"

	if err := UpdateDomain(h.db, existing); err != nil {
		log.Printf("Error updating domain: %v", err)
		h.renderDomainFormError(w, r, "Failed to update domain: "+err.Error(), true, r.Form)
		return
	}

	http.Redirect(w, r, "/domains?msg=Domain+updated+successfully", http.StatusSeeOther)
}

func (h *Handler) handleDomainDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := DeleteDomain(h.db, id); err != nil {
		log.Printf("Error deleting domain: %v", err)
		http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		return
	}

	// Nullify domain_id in reports (orphan gracefully)
	h.db.Model(&models.Report{}).Where("domain_id = ?", id).Update("domain_id", nil)

	http.Redirect(w, r, "/domains?msg=Domain+deleted+successfully", http.StatusSeeOther)
}

func (h *Handler) handleDomainToggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := ToggleDomain(h.db, id); err != nil {
		log.Printf("Error toggling domain: %v", err)
		http.Error(w, "Failed to toggle domain", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/domains", http.StatusSeeOther)
}

// --- Helper Functions ---

func (h *Handler) renderDomainFormError(w http.ResponseWriter, r *http.Request, errMsg string, isEdit bool, _ map[string][]string) {
	port, _ := strconv.Atoi(r.FormValue("imap_port"))
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)

	data := DomainFormData{
		Domain: DomainFormValues{
			ID:                uint(id),
			Name:              r.FormValue("name"),
			IMAPServer:        r.FormValue("imap_server"),
			IMAPPort:          port,
			IMAPUser:          r.FormValue("imap_user"),
			IMAPFolder:        r.FormValue("imap_folder"),
			IMAPMoveFolder:    r.FormValue("imap_move_folder"),
			IMAPMoveFolderErr: r.FormValue("imap_move_folder_err"),
			Enabled:           r.FormValue("enabled") == "on",
		},
		IsEdit: isEdit,
		Error:  errMsg,
	}

	if r.Header.Get("HX-Request") == "true" {
		h.tmpl.ExecuteTemplate(w, "domainForm", data)
	} else {
		h.tmpl.ExecuteTemplate(w, "domainFormLayout", data)
	}
}

func (h *Handler) buildPageData(r *http.Request) *PageData {
	hasData := HasAnyData(h.db)

	opts, _ := FetchFilterOptions(h.db)

	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	domains := r.URL.Query()["domains"]
	orgs := r.URL.Query()["orgs"]

	// Default date range: last 7 days
	maxDate := opts.MaxDate
	minDate := opts.MinDate

	maxDateStr := formatDate(maxDate)
	minDateStr := formatDate(minDate)

	if startDateStr == "" {
		defaultStart := maxDate.AddDate(0, 0, -7)
		if defaultStart.Before(minDate) {
			defaultStart = minDate
		}
		startDateStr = formatDate(defaultStart)
	}
	if endDateStr == "" {
		endDateStr = maxDateStr
	}

	// Default: select all domains and orgs
	selectAllDomains := len(domains) == 0 || len(domains) == len(opts.Domains)
	selectAllOrgs := len(orgs) == 0 || len(orgs) == len(opts.Orgs)

	if len(domains) == 0 {
		domains = opts.Domains
	}
	if len(orgs) == 0 {
		orgs = opts.Orgs
	}

	sidebar := SidebarData{
		MinDate:          minDateStr,
		MaxDate:          maxDateStr,
		StartDate:        startDateStr,
		EndDate:          endDateStr,
		AllDomains:       opts.Domains,
		SelectedDomains:  domains,
		SelectAllDomains: selectAllDomains,
		AllOrgs:          opts.Orgs,
		SelectedOrgs:     orgs,
		SelectAllOrgs:    selectAllOrgs,
	}

	pageData := &PageData{
		HasData: hasData,
		Sidebar: sidebar,
	}

	if !hasData || len(domains) == 0 || len(orgs) == 0 {
		return pageData
	}

	startDate, _ := time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	endDate, _ := time.ParseInLocation("2006-01-02", endDateStr, time.Local)

	metrics, err := FetchGlobalMetrics(h.db, startDate, endDate, domains, orgs)
	if err != nil {
		log.Printf("Error fetching metrics: %v", err)
	} else {
		pageData.Metrics = metrics
	}

	chartData, err := FetchTimeSeriesData(h.db, startDate, endDate, domains, orgs)
	if err != nil {
		log.Printf("Error fetching chart data: %v", err)
	} else {
		pageData.ChartData = chartData
	}

	reports, err := FetchReportsList(h.db, startDate, endDate, domains, orgs)
	if err != nil {
		log.Printf("Error fetching reports: %v", err)
	} else {
		pageData.Reports = reports
	}

	return pageData
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format("2006-01-02")
	}
	return t.Format("2006-01-02")
}

func parseID(r *http.Request) (uint, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

// basicAuth returns a middleware that enforces HTTP Basic Authentication.
func basicAuth(expectedUser, expectedPassword string) func(http.Handler) http.Handler {
	expectedUserHash := sha256.Sum256([]byte(expectedUser))
	expectedPassHash := sha256.Sum256([]byte(expectedPassword))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Inkwell Dashboard"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userHash := sha256.Sum256([]byte(user))
			passHash := sha256.Sum256([]byte(pass))

			userMatch := subtle.ConstantTimeCompare(userHash[:], expectedUserHash[:]) == 1
			passMatch := subtle.ConstantTimeCompare(passHash[:], expectedPassHash[:]) == 1

			if !userMatch || !passMatch {
				w.Header().Set("WWW-Authenticate", `Basic realm="Inkwell Dashboard"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
