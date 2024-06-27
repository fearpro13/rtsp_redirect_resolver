# rtsp_redirect_resolver
## Returns rtsp source final destination

### Problem:
RTSP RFC defines method called "REDIRECT". Despite the fact RFC was finished at April 1998 it is still possible to some RTSP clients not to support such a feature  
Program resolves final destinations of input RTSP sources  
That could be described in simple scheme:  

RTSP client without REDIRECT support will end up on Read Source1 error

    Client -> Source1 -X- Source2 -> Source3 -> Content

By using rtsp_redirect_resolver Source1 resolves to Source3 that is readable by client

    Client -> Source1 -> Source2 -> Source3 -> Content

### Build:
    make build
### Run:
    make args='<run arguments>' run

### Usage:
    rtsp_redirect_resolver <format> sources...
    
    where format is one of:
    args - prints result in single row, all final sources separated by single space, each
    nl - prints result in multiple rows, all final sources separated by newline \n
    json - print result as json array to redirect_sources.json file
    csv - print result as table to redirect_sources.csv file
    http:<port>:<refresh_interval_seconds> - returns result on HTTP API at 'GET localhost:<port>/', input sources are refreshed and resolved every <refresh_interval_seconds>
    
    supported sources:
    http|https - fetches url that contains json array and adds it to input list
    json - parses local file containing json array and adds it to input list
    rtsp|rtsps - just adds source to input list
    csv - parses local csv file and adds it to input list
    
    example:
    rtsp_redirect_resolver args rtsp://127.0.0.1/stream1 https://mybroadcast.com/broadcasts ~/local_broadcasts.json
    rtsp_redirect_resolver http:8123:3600 rtsp://127.0.0.1/stream1 https://mybroadcast.com/broadcasts ~/local_broadcasts.json

