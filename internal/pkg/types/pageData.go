package types

import "time"

// Data structure to organize and store relevant information from the page
type PageData struct {
    URL             string              `json:"url"`
    CanonicalURL    string              `json:"canonical_url"`
    Title           string              `json:"title"`
    Charset         string              `json:"charset"`
    MetaDescription string              `json:"meta_description"`
    MetaKeywords    string              `json:"meta_keywords"`
    Language        string              `json:"language"`
    Headings        map[string][]string `json:"headings"`
    AltTexts        []string            `json:"alt_texts"`
    AnchorTexts     []string            `json:"anchor_texts"`
    InternalLinks   []string            `json:"internal_links"`
    ExternalLinks   []string            `json:"external_links"`
    StructuredData  []string            `json:"structured_data"`
    OpenGraph       map[string]string   `json:"open_graph"`
    DatePublished   time.Time           `json:"date_published"`
    DateModified    time.Time           `json:"date_modified"`
    SocialLinks     []string            `json:"social_links"`
    VisibleText     string              `json:"visible_text"`
    LoadTime        time.Duration       `json:"load_time"`
    IsSecure        bool                `json:"is_secure"`
    FetchError      string              `json:"fetch_error"`
}
