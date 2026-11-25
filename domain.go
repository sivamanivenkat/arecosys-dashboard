package main

type Source struct {
	State string `json:"state"`
	Data  any    `json:"data,omitempty"`
}

type Lead struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Company   string     `json:"company"`
	Interest  string     `json:"interest"`
	Source    string     `json:"source"`
	Status    string     `json:"status"`
	Note      string     `json:"note"`
	Campaign  string     `json:"campaign"`
	Created   string     `json:"created"`
	Assignee  string     `json:"assignee"`
	Responded string     `json:"responded"`
	Phone     string     `json:"phone"`
	Activity  []Activity `json:"activity"`
}

// Activity is one entry in a lead's timeline: a note or an automatic change record.
type Activity struct {
	At   string `json:"at"`
	By   string `json:"by"`
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
}
