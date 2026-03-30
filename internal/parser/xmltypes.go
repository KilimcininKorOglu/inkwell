package parser

import "encoding/xml"

// Feedback is the root element of a DMARC aggregate report XML.
// Go's encoding/xml automatically handles single vs multiple elements
// into slices, eliminating the xmltodict dict/list quirk from Python.
type Feedback struct {
	XMLName         xml.Name        `xml:"feedback"`
	ReportMetadata  ReportMetadata  `xml:"report_metadata"`
	PolicyPublished PolicyPublished `xml:"policy_published"`
	Records         []RecordXML     `xml:"record"`
}

type ReportMetadata struct {
	OrgName   string    `xml:"org_name"`
	Email     string    `xml:"email"`
	ReportID  string    `xml:"report_id"`
	DateRange DateRange `xml:"date_range"`
}

type DateRange struct {
	Begin int64 `xml:"begin"`
	End   int64 `xml:"end"`
}

type PolicyPublished struct {
	Domain string `xml:"domain"`
	Adkim  string `xml:"adkim"`
	Aspf   string `xml:"aspf"`
	P      string `xml:"p"`
	Sp     string `xml:"sp"`
	Pct    string `xml:"pct"`
}

type RecordXML struct {
	Row         RowXML         `xml:"row"`
	Identifiers IdentifiersXML `xml:"identifiers"`
	AuthResults AuthResultsXML `xml:"auth_results"`
}

type RowXML struct {
	SourceIP        string          `xml:"source_ip"`
	Count           int             `xml:"count"`
	PolicyEvaluated PolicyEvaluated `xml:"policy_evaluated"`
}

type PolicyEvaluated struct {
	Disposition string      `xml:"disposition"`
	DKIM        string      `xml:"dkim"`
	SPF         string      `xml:"spf"`
	Reasons     []ReasonXML `xml:"reason"`
}

type ReasonXML struct {
	Type    string `xml:"type"`
	Comment string `xml:"comment"`
}

type IdentifiersXML struct {
	HeaderFrom string `xml:"header_from"`
}

type AuthResultsXML struct {
	DKIM []DKIMResultXML `xml:"dkim"`
	SPF  []SPFResultXML  `xml:"spf"`
}

type DKIMResultXML struct {
	Domain   string `xml:"domain"`
	Result   string `xml:"result"`
	Selector string `xml:"selector"`
}

type SPFResultXML struct {
	Domain string `xml:"domain"`
	Result string `xml:"result"`
}
