package dashboard

// PageData is the top-level struct passed to layout.html.
type PageData struct {
	HasData        bool
	SearchQuery    string
	Sidebar        SidebarData
	Metrics        *MetricsData
	Reports        []ReportRow
	AllDomains     []string
	SelectedDomain string
}

// SidebarData holds filter state for rendering.
type SidebarData struct {
	MinDate          string
	MaxDate          string
	StartDate        string
	EndDate          string
	AllDomains       []string
	SelectedDomains  []string
	SelectAllDomains bool
	AllOrgs          []string
	SelectedOrgs     []string
	SelectAllOrgs    bool
}

// MetricsData corresponds to the 3-column metrics.
type MetricsData struct {
	TotalIPs    int
	TotalVolume int64
	PassRateStr string
}

// ReportRow is one row in the master reports table.
type ReportRow struct {
	DBID         uint
	StartDate    string
	EndDate      string
	Domain       string
	Organization string
	ReportID     string
	Messages     int64
	Adkim        string
	Aspf         string
	P            string
	Sp           string
	Pct          int
}

// ReportDetailData is the expanded detail view for a selected report.
type ReportDetailData struct {
	DBID         uint
	Organization string
	Domain       string
	StartDate    string
	EndDate      string
	Adkim        string
	Aspf         string
	P            string
	Sp           string
	Pct          int
	Records      []DetailRecord
}

// DetailRecord is one row in the IP stats aggregation table.
type DetailRecord struct {
	SourceIP    string
	HostName    string
	Count       int
	Disposition string
	Reason      string
	DKIMDomain  string
	DKIMAuth    string
	SPFDomain   string
	SPFAuth     string
	DKIMAlign   string
	SPFAlign    string
	DMARC       string
}

// --- Domain Management ViewModels ---

// DomainsPageData is the top-level struct for the /domains page.
type DomainsPageData struct {
	Domains   []DomainRow
	Message   string
	CSRFToken string
}

// DomainRow is one row in the domains list table.
type DomainRow struct {
	ID         uint
	Name       string
	IMAPServer string
	IMAPUser   string
	IMAPFolder string
	Enabled    bool
	CreatedAt  string
}

// DomainFormData is passed to the domain add/edit form template.
type DomainFormData struct {
	Domain    DomainFormValues
	IsEdit    bool
	Error     string
	CSRFToken string
}

// DomainFormValues holds the form field values for a domain.
type DomainFormValues struct {
	ID                uint
	Name              string
	IMAPServer        string
	IMAPPort          int
	IMAPUser          string
	IMAPPassword      string
	IMAPFolder        string
	IMAPMoveFolder    string
	IMAPMoveFolderErr string
	Enabled           bool
}

// --- Login ViewModels ---

// LoginData is passed to the login page template.
type LoginData struct {
	Username  string
	Error     string
	CSRFToken string
}
