package summary

import (
	"sort"
	"time"

	"github.com/taupikpirdian/wlog/domain/dashboard"
)

type TicketDay struct {
	Key string
	Day Day
}

func FilterTicket(source dashboard.Snapshot, key string) dashboard.Snapshot {
	var filtered dashboard.Snapshot
	for _, s := range source.Sessions {
		if s.TicketKey == key {
			filtered.Sessions = append(filtered.Sessions, s)
		}
	}
	for _, a := range source.Activities {
		if a.TicketKey == key {
			filtered.Activities = append(filtered.Activities, a)
		}
	}
	return filtered
}

func TicketsForDay(source dashboard.Snapshot, date, now time.Time, location *time.Location) ([]TicketDay, error) {
	keys := map[string]bool{}
	for _, s := range source.Sessions {
		if s.TicketKey != "" {
			keys[s.TicketKey] = true
		}
	}
	for _, a := range source.Activities {
		if a.TicketKey != "" {
			keys[a.TicketKey] = true
		}
	}
	var result []TicketDay
	for key := range keys {
		day, err := BuildDay(FilterTicket(source, key), date, now, location)
		if err != nil {
			return nil, err
		}
		if day.HasWorklog {
			result = append(result, TicketDay{Key: key, Day: day})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

// TicketsForWeek lists each ticket once and totals only its current-week work.
func TicketsForWeek(source dashboard.Snapshot, now time.Time, location *time.Location) ([]TicketDay, error) {
	dates, err := WeekDates(now, location)
	if err != nil {
		return nil, err
	}
	byKey := map[string]TicketDay{}
	for _, date := range dates {
		tickets, err := TicketsForDay(source, date, now, location)
		if err != nil {
			return nil, err
		}
		for _, ticket := range tickets {
			total := byKey[ticket.Key]
			total.Key = ticket.Key
			total.Day.Date = dates[0]
			total.Day.HasWorklog = true
			total.Day.Seconds += ticket.Day.Seconds
			total.Day.CommitCount += ticket.Day.CommitCount
			byKey[ticket.Key] = total
		}
	}
	tickets := make([]TicketDay, 0, len(byKey))
	for _, ticket := range byKey {
		tickets = append(tickets, ticket)
	}
	sort.Slice(tickets, func(i, j int) bool { return tickets[i].Key < tickets[j].Key })
	return tickets, nil
}
