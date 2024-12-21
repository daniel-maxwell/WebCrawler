package fetcher

import (
    "time"
)

// Data structure to store and organize relevant information from the page
type PageData struct {
    URL             	string
    CanonicalURL    	string
    Title           	string
    Charset         	string
    MetaDescription 	string
    MetaKeywords    	string
    RobotsMeta      	string
    Language        	string
    Headings        	map[string][]string
    AltTexts        	[]string
    AnchorTexts     	[]string
    InternalLinks   	[]string
    ExternalLinks   	[]string
    StructuredData  	[]string
    OpenGraph       	map[string]string
    Author          	string
    DatePublished   	time.Time
    DateModified    	time.Time
    Categories      	[]string
    Tags            	[]string
    SocialLinks     	[]string
    VisibleText     	string
    LoadTime        	time.Duration
    IsSecure        	bool
    LastCrawled     	time.Time
}
