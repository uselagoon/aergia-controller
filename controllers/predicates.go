package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NamespacePredicates defines the funcs for predicates
type NamespacePredicates struct {
	predicate.Funcs
}

// Create we only watch for create events at this stage.
func (NamespacePredicates) Create(e event.CreateEvent) bool {
	return true
}

// Delete returns false if a delete event.
func (NamespacePredicates) Delete(e event.DeleteEvent) bool {
	return true
}

// Update returns false if a delete event.
func (NamespacePredicates) Update(e event.UpdateEvent) bool {
	return true
}

// Generic returns false if a delete event.
func (NamespacePredicates) Generic(e event.GenericEvent) bool {
	return true
}
