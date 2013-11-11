package main

import (
  "bufio"
  "fmt"
  "github.com/russross/blackfriday"
  "html/template"
  "io/ioutil"
  "os"
  "path"
  "path/filepath"
  "strconv"
  "strings"
  "time"
  "sync"
  "runtime"
  ttmpl "text/template"
  "reflect"
)

/*Settings*/

var POST_DIR      = "./_posts" /*no ending slash*/
var CACHE         = "./_cache" /*no ending slash*/
var POST_PER_PAGE = 10
var POST_FEED     = 10

/*Parse all the Templates*/
var SINGLE_TPL, _ = template.ParseFiles("./_layouts/single.html")
var PAGE_TPL, _ = template.ParseFiles("./_layouts/page.html")
var FEED_TPL, _ = ttmpl.ParseFiles("./_layouts/feed.xml")
var SITEMAP_TPL, _ = ttmpl.ParseFiles("./_layouts/sitemap.xml")
var ARCHIVE_TPL, _ = template.ParseFiles("./_layouts/archive.html")

/*Real code starts here*/

type Post struct {
  Day string
  filename string
  Long_month string
  Month string
  Mtime time.Time
  path string
  Title template.HTML
  Url string
  Year  string
  Content template.HTML
  ContentCut template.HTML
}
type Year struct {
  Value string
  Months []Month
}
type Month struct {
  Value string
  Posts []Post
}
type Page struct {
  Next string
  Previous string
  index int
  Posts []Post
}

var POSTS []Post
var ARCHIVES []Year
var URL map[string]int
var PAGE Page
var NOW = time.Now()
var BLACKFRIDAY_EXT = blackfriday.EXTENSION_NO_INTRA_EMPHASIS | blackfriday.EXTENSION_FENCED_CODE | blackfriday.EXTENSION_TABLES
var BLACKFRIDAY_HTML_FLAGS = blackfriday.HTML_USE_XHTML | blackfriday.HTML_USE_SMARTYPANTS | blackfriday.HTML_SMARTYPANTS_FRACTIONS
var BLACKFRIDAY_RENDERER = blackfriday.HtmlRenderer(BLACKFRIDAY_HTML_FLAGS, "", "")
var WG sync.WaitGroup

func RenderContent(path string) (title template.HTML, content template.HTML){

  input, _ := ioutil.ReadFile(path)
  /*Get the two first line and that's our title.*/
  idx := 0
  for input[idx] != '\n' && input[idx] != '\r' {
    idx++
  }

  return template.HTML(input[:idx]),
  template.HTML(blackfriday.Markdown(input[(idx<<1)+1:], BLACKFRIDAY_RENDERER, BLACKFRIDAY_EXT))
}

func CreatePage(data interface{}, template_page interface{}, page_path string) {
    if _, err := os.Stat(page_path); os.IsNotExist(err) {
      base_path := path.Dir(page_path)
      os.MkdirAll(base_path, 0777)
    }

    fo, _ := os.Create(page_path)
    document := bufio.NewWriter(fo)
    page_data := map[string]interface{}{"data": data, "now": NOW}

    vf := reflect.ValueOf(template_page).Elem().Interface()
    switch vf.(type) {
      case *template.Template:
        vf.(*template.Template).Execute(document, page_data);
      case *ttmpl.Template:
        vf.(*ttmpl.Template).Execute(document, page_data);
    }

    document.Flush()
    fo.Close()
    runtime.Gosched()
    WG.Done()
}

func visit(fpath string, f os.FileInfo, err error) error {
  if f.IsDir() {
    return nil
  }

  cur_post := new(Post)
  cur_post.path = fpath

  cur_post.Title, cur_post.Content = RenderContent(cur_post.path)
  cur_post.Mtime = f.ModTime()
  cur_post.filename = path.Base(fpath)

  data := strings.Split(cur_post.filename, "-")
  cur_post.Year = data[0]
  cur_post.Month = data[1]
  cur_post.Day = data[2]
  i_month, _ := strconv.Atoi(cur_post.Month)
  cur_post.Long_month = time.Month(i_month).String()

  /*We must check if this url already exists in POSTS, add -%d to it*/
  cur_post.Url = strings.Split(strings.Join(data[3:], "-"), ".")[0]
  URL[cur_post.Url]++
  if idx := URL[cur_post.Url]; idx > 1 {
    cur_post.Url = fmt.Sprintf("%s-%d", cur_post.Url, idx - 1)
  }

  cur_post.ContentCut = cur_post.Content
  if cut_idx := strings.Index(string(cur_post.Content), "<!--more-->"); cut_idx != -1 {
    full_link := fmt.Sprintf("</br><a href=\"/%s\">Continue reading &raquo;</a></br>", cur_post.Url)
    temp_content := cur_post.Content[:cut_idx]
    temp_content = template.HTML(fmt.Sprintf("%s%s", string(temp_content), full_link))
    cur_post.ContentCut = template.HTML(temp_content)
  }

  WG.Add(1)
  go CreatePage(cur_post, &SINGLE_TPL, fmt.Sprintf("%s/%s/index.html", CACHE, cur_post.Url))

  if len(POSTS) > 1 && (len(POSTS) - 1) % POST_PER_PAGE == 0 {
    PAGE.index++
    PAGE.Previous = fmt.Sprintf("/page/%d/", PAGE.index - 1)
    PAGE.Next = fmt.Sprintf("/page/%d/", PAGE.index + 1)
    if PAGE.index == 1 {
      PAGE.Previous = ""
      PAGE.Next = "/page/2/"
    }
    max := len(POSTS) - 1
    min := max - POST_PER_PAGE
    /*Loop through min and max, index POSTS backward and append to PAGE.Posts*/
    PAGE.Posts = make([]Post, 0)
    for idx := max; idx > min; idx-- {
      PAGE.Posts = append(PAGE.Posts, POSTS[idx])
    }
    WG.Add(1)
    go CreatePage(PAGE, &PAGE_TPL, fmt.Sprintf("%s/page/%d/index.html", CACHE, PAGE.index))
  }

  POSTS = append(POSTS, *cur_post)
  return nil
}

func main() {
  /*You want CPU + 1 so even when all the threads are busy doing IO, it can go on*/
  runtime.GOMAXPROCS(runtime.NumCPU()+1)

  fmt.Println("Start")
  fmt.Println("Generating files...")

  URL = make(map[string]int)
  err := filepath.Walk(POST_DIR, visit)
  if err != nil {
    fmt.Printf("error listing posts: %v\n",err)
    os.Exit(1)
  }

  len_posts := len(POSTS) - 1

  /*Create the last page*/
  max := len(POSTS) - 1
  min := (PAGE.index * POST_PER_PAGE) - 1
  PAGE.index++
  PAGE.Previous = fmt.Sprintf("/page/%d/", PAGE.index - 1)
  PAGE.Next = ""
  PAGE.Posts = make([]Post, 0)
  for idx := max; idx > min; idx-- {
    PAGE.Posts = append(PAGE.Posts, POSTS[idx])
  }
  WG.Add(1)
  go CreatePage(PAGE, &PAGE_TPL, fmt.Sprintf("%s/page/%d/index.html", CACHE, PAGE.index))
  /*Create a link to the last page*/
  os.Symlink(fmt.Sprintf("./page/%d/index.html", PAGE.index), fmt.Sprintf("%s/index.html", CACHE))

  /*Build the archive*/
  var current *Year
  var months *Month
  for i := len_posts; i >= 0; i-- {
    a := POSTS[i]
    if current == nil || current.Value != a.Year {
      if current == nil {
        current = new(Year)
        months = new(Month)
      }else{
        current.Months = append(current.Months, *months)
        ARCHIVES = append(ARCHIVES, *current)
        current.Months = make([]Month, 0)
      }
      current.Value = a.Year
      months.Value = a.Long_month
    }

    if months.Value != a.Long_month {
      current.Months = append(current.Months, *months)
      months.Value = a.Long_month
      months.Posts = make([]Post, 0)
    }
    months.Posts = append(months.Posts, a)
  }
  current.Months = append(current.Months, *months)
  ARCHIVES = append(ARCHIVES, *current)

  WG.Add(1)
  fmt.Println("Generating archive...")
  go CreatePage(ARCHIVES, &ARCHIVE_TPL, fmt.Sprintf("%s/archive.html", CACHE))

  WG.Add(1)
  fmt.Println("Generating feed...")
  go CreatePage(POSTS[len(POSTS)-POST_FEED:], &FEED_TPL, fmt.Sprintf("%s/feed.xml", CACHE))

  WG.Add(1)
  fmt.Println("Generating sitemap...")
  go CreatePage(POSTS, &SITEMAP_TPL, fmt.Sprintf("%s/sitemap.xml", CACHE))

  WG.Wait()
  fmt.Println("Stop")

}
