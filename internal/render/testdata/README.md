# Test employees

The employees under `employees/` are made up for this repository's tests. They are not catalog employees and carry no
text from hr.harnsy.dev: this repository holds code only. They have the shapes the catalog has: no skills
(`harnsy/sample-plain`), skills (`harnsy/sample-skills`), skills with scripts and their licence
(`harnsy/sample-scripts`), and an employee of another publisher under CC0 (`octo/sample-guest`).

`golden/<target>/` is what the renderer must write for each of them. After a deliberate change of the renderer:
`go test ./internal/render -update`, then read the diff.
