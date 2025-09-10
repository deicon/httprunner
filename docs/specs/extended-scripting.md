# Extended Scripting

## Request Syntax
Requests are defined in a `.http` file using the following syntax:

```http
###
# @name <Request Name>
# @BeforeUser
# @BeforeIteration
# @TeardownUser
# @TeardownIteration
# Request description (optional)
> {%
<JavaScript_Code>
%}

<HTTP_VERB> <URL>
<Header-Name>: <Header-Value>
<Header-Name>: <Header-Value>   

<JSON_BODY>
> {%
<JavaScript_Code>
%}

```
### Key Points:
- Requests Names, if present need to be unique within the file.
- Requests are separated by `###`.
- The request body is optional and can be omitted for requests without a body
- JavaScript code blocks are optional and can be placed before the request line (pre-request script) or after the request body (post-request script).
- Blank lines are used to separate different sections of the request.
- Comments can be added using `#` at the beginning of a line.
- The `@BeforeUser` and `@BeforeIteration` annotations can be used to specify scripts that run once per Virtual user or once before each iteration, respectively.
- The `@TeardownUser` and `@TeardownIteration` annotations can be used to specify scripts that run once after all iterations for a Virtual user or once after each iteration, respectively.
- requests can contain only one javascript code block. This is useful if used with `@BeforeUser` or `@BeforeIteration` and `@TeardownUser` and `@TeardownIteration`  annotations.

### Code Generation
Each named Request is converted into a JavaScript function and available within the scripting context.
The function name is derived from the request name by replacing spaces with underscores and converting to lowercase.
For example, a request named "Create User" becomes a function named `create_user()`.

### Example Request with Template, Scripting Annotations and Code Generation using https://jsonplaceholder.typicode.com/
```http
###
# @name Create Post
# @BeforeUser
# This request creates a new post

> {%
    // Pre-request script: Set up any necessary variables or state
    client.global.set("postTitle", "foo");
    client.global.set("postBody", "bar");
    client.global.set("postUserId", 1);
%}

POST https://jsonplaceholder.typicode.com/posts
Content-Type: application/json

{
  "title": "{{.postTitle}}",
  "body": "{{.postBody}}",
  "userId": {{.postUserId}}
}

> {%
    // Post-request script: Process the response and store values in global variables
    var jsonData = response.body;
    client.global.set("createdPostId", jsonData.id);
%} 

###
# @name Get Post
# This request retrieves the created post

GET https://jsonplaceholder.typicode.com/posts/{{.createdPostId}}
Accept: application/json

> {%
// Post-request script: Process the response
var jsonData = response.body;
console.log("Retrieved Post Title: " + jsonData.title);
%}

###
# @name Get Post using Generated Function

> {%
    var response = client.get_post();
%}

```

    