package fetcher

import (
    "time"
)

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
    IsMobileFriendly 	bool
    LastCrawled     	time.Time
}

