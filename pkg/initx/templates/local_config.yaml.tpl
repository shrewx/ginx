server:
  id: 1
  name: {{ .ProjectName }}
  host: 127.0.0.1
  port: 8321
  show_params: true
  i18n:
    langs: ["zh", "en"]
    unmarshal_type: "yaml"
    paths: []

db:
  type: sqlite
  dbname: test.db

log:
  filename: {{ .ProjectName }}.log
  dir_path: logs
  log_level: debug
  maxsize: 10
  maxbackups: 5
  compress: true
  to_stdout: true
  enable_caller: true




