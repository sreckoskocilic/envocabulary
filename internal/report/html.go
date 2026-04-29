package report

import (
	"html/template"
	"io"
)

var htmlTmpl = template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>envocabulary audit report</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, 'Helvetica Neue', sans-serif;
    background: #1c1c1e; color: #e0e0e0;
    padding: 3rem 4rem; line-height: 1.7; font-size: 14px;
    max-width: 960px; margin: 0 auto;
  }
  header { margin-bottom: 3rem; }
  header h1 {
    font-size: 1.1rem; font-weight: 400;
    letter-spacing: 0.15em; text-transform: uppercase; color: #777;
    margin-bottom: 0.2rem;
  }
  header .meta { font-size: 0.8rem; color: #555; }
  section { margin-bottom: 2.5rem; }
  .section-header {
    cursor: pointer; user-select: none;
    display: flex; align-items: baseline; gap: 0.75rem;
    padding-bottom: 0.4rem; border-bottom: 2px solid #e0e0e0;
  }
  .section-header h2 {
    font-size: 0.85rem; font-weight: 600;
    text-transform: uppercase; letter-spacing: 0.08em;
  }
  .section-header .count { font-size: 0.8rem; font-weight: 400; color: #666; }
  .collapsed .section-body { display: none; }
  .collapsed .section-header { border-bottom-color: #444; }
  table { width: 100%; border-collapse: collapse; }
  th {
    text-align: left; padding: 0.5rem 0;
    font-size: 0.7rem; font-weight: 600;
    text-transform: uppercase; letter-spacing: 0.06em;
    color: #666; border-bottom: 1px solid #333;
  }
  td {
    padding: 0.45rem 0; border-bottom: 1px solid #2a2a2c; font-size: 0.85rem;
  }
  td:first-child { padding-right: 1.5rem; }
  td:nth-child(2) { padding-right: 1.5rem; }
  tr:last-child td { border-bottom: none; }
  .def { font-family: 'SF Mono', 'Menlo', monospace; font-size: 0.8rem; }
  .loc { font-family: 'SF Mono', 'Menlo', monospace; font-size: 0.8rem; color: #888; }
  .superseded { font-family: 'SF Mono', 'Menlo', monospace; font-size: 0.8rem; color: #6dba5a; }
  .review-ref { font-family: 'SF Mono', 'Menlo', monospace; font-size: 0.8rem; color: #c4a860; }
  .review-active-val { color: #666; font-size: 0.78rem; }
  .review-active-val::before { content: "\2192  "; }
  .missing { font-family: 'SF Mono', 'Menlo', monospace; font-size: 0.8rem; color: #d46; }
  .orphan-path { font-family: 'SF Mono', 'Menlo', monospace; font-size: 0.8rem; }
  .orphan-detail { color: #666; font-size: 0.8rem; }
  .empty { padding: 1rem 0; color: #555; }
  @media (max-width: 700px) { body { padding: 1.5rem; } }
</style>
</head>
<body>

<header>
  <h1>envocabulary</h1>
  <div class="meta">audit report &middot; {{.Generated.Format "2006-01-02 15:04"}} &middot; {{.FilesScanned}} files scanned</div>
</header>

<section id="safe">
  <div class="section-header" onclick="this.parentElement.classList.toggle('collapsed')">
    <h2>safe to delete</h2>
    <span class="count">{{len .Safe}}</span>
  </div>
  <div class="section-body">
{{- if .Safe}}
    <table>
      <thead><tr><th>definition</th><th>location</th><th>superseded by</th></tr></thead>
      <tbody>
{{- range .Safe}}
        <tr>
          <td><span class="def">{{.Definition}}</span></td>
          <td class="loc">{{.Location}}</td>
          <td class="superseded">{{.Reference}}</td>
        </tr>
{{- end}}
      </tbody>
    </table>
{{- else}}
    <div class="empty">none</div>
{{- end}}
  </div>
</section>

<section id="review">
  <div class="section-header" onclick="this.parentElement.classList.toggle('collapsed')">
    <h2>review</h2>
    <span class="count">{{len .Review}}</span>
  </div>
  <div class="section-body">
{{- if .Review}}
    <table>
      <thead><tr><th>definition</th><th>location</th><th>superseded by</th></tr></thead>
      <tbody>
{{- range .Review}}
        <tr>
          <td><span class="def">{{.Definition}}</span></td>
          <td class="loc">{{.Location}}</td>
          <td><span class="review-ref">{{.Reference}}</span>{{if .ActiveValue}} <span class="review-active-val">{{.ActiveValue}}</span>{{end}}</td>
        </tr>
{{- end}}
      </tbody>
    </table>
{{- else}}
    <div class="empty">none</div>
{{- end}}
  </div>
</section>

<section id="dangling">
  <div class="section-header" onclick="this.parentElement.classList.toggle('collapsed')">
    <h2>dangling</h2>
    <span class="count">{{len .Dangling}}</span>
  </div>
  <div class="section-body">
{{- if .Dangling}}
    <table>
      <thead><tr><th>definition</th><th>location</th><th>missing target</th></tr></thead>
      <tbody>
{{- range .Dangling}}
        <tr>
          <td><span class="def">{{.Definition}}</span></td>
          <td class="loc">{{.Location}}</td>
          <td class="missing">{{.Reference}}</td>
        </tr>
{{- end}}
      </tbody>
    </table>
{{- else}}
    <div class="empty">none</div>
{{- end}}
  </div>
</section>

<section id="orphaned">
  <div class="section-header" onclick="this.parentElement.classList.toggle('collapsed')">
    <h2>orphaned files</h2>
    <span class="count">{{len .Orphans}}</span>
  </div>
  <div class="section-body">
{{- if .Orphans}}
    <table>
      <thead><tr><th>file</th><th>contents</th></tr></thead>
      <tbody>
{{- range .Orphans}}
        <tr>
          <td class="orphan-path">{{.Path}}</td>
          <td class="orphan-detail">{{.Summary}}</td>
        </tr>
{{- end}}
      </tbody>
    </table>
{{- else}}
    <div class="empty">none</div>
{{- end}}
  </div>
</section>

</body>
</html>
`))

func WriteHTML(w io.Writer, r Report) error {
	return htmlTmpl.Execute(w, r)
}
