package main

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	log "github.com/sirupsen/logrus"
)

func (v *visitor) addFilter(m []*labels.Matcher) []*labels.Matcher {
	m = append(m, v.filter...)
	log.WithFields(log.Fields{"matchers": m}).Debug("addFilter")
	return m
}

type visitor struct {
	filter []*labels.Matcher
}

func newVisitor(filter string) (*visitor, error) {
	f, err := promql.ParseMetricSelector(filter)
	if err != nil {
		return nil, err
	}
	v := &visitor{
		filter: f,
	}
	return v, nil
}

func (v *visitor) Visit(node promql.Node) (w promql.Visitor) {
	if node == nil {
		return
	}
	log.WithFields(log.Fields{"node": node}).Debug("Visit")
	switch n := node.(type) {
	case *promql.VectorSelector:
		n.LabelMatchers = v.addFilter(n.LabelMatchers)
	case *promql.MatrixSelector:
		n.LabelMatchers = v.addFilter(n.LabelMatchers)
	case *promql.BinaryExpr:
	case *promql.Call:
	case *promql.AggregateExpr:
	case *promql.NumberLiteral:
	case *promql.ParenExpr:
	case promql.Expressions:
	default:
		log.Warnf("Unknown type %T", n)
	}
	return v
}

func addQueryFilter(filter, query string) (string, error) {
	expr, err := promql.ParseExpr(query)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Debug("ParseExpr")
		return "", fmt.Errorf("promql parse error: %s", err)
	}
	log.Debug(promql.Tree(expr))
	v, err := newVisitor(filter)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("failed to create visitor")
		return "", err
	}
	promql.Walk(v, expr)
	newQuery := expr.String()
	log.WithFields(log.Fields{"origQuery": query, "newQuery": newQuery}).Debug("rewrote query")
	return newQuery, nil
}
