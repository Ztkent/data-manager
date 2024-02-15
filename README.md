# Data Manager
Crawl, collect, visualize, and extract data from from across the web.

## About
Data Manager is a powerful tool to simplify data collection and processing.  
It offers a convenient way to quickly crawl and collect recent web data for data models or research.

## Features
    - Crawl and collect data from the web
    - Visualize and extract data
    - Process and store data
    - Export data to SQLite or CSV

## Infrastructure
    - Frontend: HTML, TailwindCSS, HTMX
    - Backend: Go
    - Services: Rust (Data-Crawler), Python (Data-Processor)


// TODO: 
- require login for crawl/random/export/visualize
- serve a strict robots.txt. Bing already found us. Likely the github ref link.
- Fix the Cert. Its for ztkent.com, not data-manager.ztkent.com
- add collected files tab
- cleanup, remove and files for users not active in db for 3 days
