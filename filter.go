package main

import (
	"fmt"

	"github.com/prometheus/prometheus/model/labels"
	promql "github.com/prometheus/prometheus/promql/parser"
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

func (v *visitor) Visit(node promql.Node, path []promql.Node) (promql.Visitor, error) {
	if node == nil {
		return v, nil
	}
	log.WithFields(log.Fields{"node": node}).Debug("Visit")
	switch n := node.(type) {
	case *promql.VectorSelector:
		n.LabelMatchers = v.addFilter(n.LabelMatchers)
	case *promql.MatrixSelector:
		// VectorSelector is an Expr and is treated as a child, therefore
		// we do not have to explicitly handle it here.
	case *promql.AggregateExpr:
	case *promql.BinaryExpr:
	case *promql.Call:
	case promql.Expressions:
	case *promql.NumberLiteral:
	case *promql.StringLiteral:
	case *promql.SubqueryExpr:
	case *promql.ParenExpr:
	case *promql.UnaryExpr:
	default:
		log.Warnf("Unknown type %T", n)
	}
	return v, nil
}

func addQueryFilter(filter, query string) (string, error) {
	var expr promql.Expr
	if query == "{}" {
		expr = &promql.VectorSelector{
			BypassEmptyMatcherCheck: true,
		}
	} else {
		var err error
		expr, err = promql.ParseExpr(query)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Debug("ParseExpr")
			return "", fmt.Errorf("promql parse error: %s", err)
		}
	}
	log.Debug(promql.Tree(expr))
	v, err := newVisitor(filter)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("failed to create visitor")
		return "", err
	}
	var path []promql.Node
	promql.Walk(v, expr, path)
	newQuery := expr.String()
	log.WithFields(log.Fields{"origQuery": query, "newQuery": newQuery}).Debug("rewrote query")
	return newQuery, nil
}
