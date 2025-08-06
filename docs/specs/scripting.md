# Template and Scripting

In order to create dynamic requests which can use variables, you can embed JavaScript code in the request file.
This allows you to generate parts of the request dynamically based on previous responses or other logic.

Therefore all requests should be trated as templates, and rendered before execution replacing all variables with their
values.
All ENV Variables should be available in the template rendering context.

# Template Syntax

The template syntax is based on the [Standard Text template](https://pkg.go.dev/text/template) package in Go.
After the request body, you can embed JavaScript code blocks using the following syntax:

to retrieve and set variables, you can use the `client.global` object which is available in the context and contains all global variables starting
with all environment variables.

```javascript
> {%
var jsonData = response.body
//
client.global.set("tarifrechnerId", jsonData.id)
%}
```


# Example Request with Template
###
# @name Tarifrechner anlegen
POST {{.BASEURL}}//api/{{.APIVERSION}}/tarifrechner
Authorization: Bearer {{.TOKEN}}
Content-Type: application/json

{
"tarifrechnerModus": {
"modus": "{{.MODUS}}",
"mandant": "DVAG",
"haushaltsId": "{{.HAUSHALTSID-6905550}}"
},
"produktKonfigurationId": "Kontoeroeffnung",
"kundennummern": [
    "99152160"
]
}


> {%

var jsonData = response.body
//
client.global.set("tarifrechnerId", jsonData.id)
client.global.set("beteiligter_1", jsonData.beteiligte[0].id)

%}

