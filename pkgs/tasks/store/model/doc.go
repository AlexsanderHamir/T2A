// Package model holds GORM persistence models for the tasks domain.
// It owns schema tags, TableName methods, and domain ⇄ model mapping.
// It imports domain; domain never imports model. It must not import
// postgres or store.
package model
