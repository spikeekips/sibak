// GENERATED BY THE COMMAND ABOVE; DO NOT EDIT
// This file was generated by swaggo/swag at
// 2018-08-31 02:16:27.172371588 +0900 KST m=+0.117131970

package docs

import (
	"github.com/swaggo/swag"
)

var doc = `{
    "swagger": "2.0",
    "info": {
        "title": "API",
        "contact": {},
        "license": {},
        "version": "1.0"
    },
    "basePath": "/api",
    "paths": {
        "/account/{address}": {
            "get": {
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "parameters": [
                    {
                        "type": "string",
                        "description": "Account's address",
                        "name": "address",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "$ref": "#/definitions/block.BlockAccount"
                        }
                    },
                    "500": {}
                }
            }
        }
    },
    "definitions": {
        "block.BlockAccount": {
            "type": "object",
            "properties": {
                "address": {
                    "type": "string"
                },
                "balance": {
                    "type": "string"
                },
                "checkpoint": {
                    "type": "string"
                },
                "codeHash": {
                    "type": "array",
                    "items": {
                        "type": "byte"
                    }
                },
                "rootHash": {
                    "type": "object",
                    "$ref": "#/definitions/sebakcommon.Hash"
                }
            }
        },
        "sebakcommon.Hash": {
            "type": "object"
        }
    }
}`

type s struct{}

func (s *s) ReadDoc() string {
	return doc
}
func init() {
	swag.Register(swag.Name, &s{})
}
