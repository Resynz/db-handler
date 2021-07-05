package db_handler

type Condition struct {
	Where  string        `json:"where"`  //  "user=? and status=?"
	Params []interface{} `json:"params"` // [ 1,0]
	Desc   []string      `json:"desc"`
	Asc    []string      `json:"asc"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}
