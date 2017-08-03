# Blog API

A simple and unsecure API to store blog data. The server limit the number of request
from an IP at 264 per minute.

## Store Article

Add an article in the database.

- **URL**: 

    /article/{id}/

- **Method**:

    POST

- **URL Param**:

    **required**: </br>
    `id=[string]` represents an user ID 

- **Data Param**:

    ```json
    {
        "title": "My Article",
        "content": "Whatever I want to say!"
    }
    ```

- **Success Response**: 

    **Code**: `200 OK` </br>
    **Content**:
    ```json
    {
        "title": "My Article",
        "content": "Whatever I want to say!"
    }
    ```

- **Error Response**: 

    **Code**: `400 Bad Request` </br>
    **Content**: `error as plain/text`

    **Code**: `500 Internal Server Error` </br>
    **Content**: `error as plain/text`

## Get Article

Get an article from the database.

- **URL**: 

    /article/{id}/{title}

- **Method**:

    GET

- **URL Param**:

    **required**: </br>
    `id=[string]` represent an user ID </br>
    `title=[string]` represent the title of an article

- **Data Param**:

    None

- **Success Response**: 

    **Code**: `200 OK` </br>
    **Content**: 
    ```json
    {
        "title": "My Article",
        "content": "Whatever I want to say!"
    }
    ```

## Delete Article

Delete an article from the database.

- **URL**:

    /article/{id}/{title}

- **Method**:

    DELETE

- **URL Param**:

    **required**: </br>
    `id=[string]` represent an user ID </br>
    `title=[string]` represent the title of an article

- **Data Param**:

    None

- **Success Response**: 

    **Code**: `200 OK` </br>
    **Content**: None

- **Error Response**: 

    **Code**: `400 Bad Request` </br>
    **Content**: `error as plain/text`

    **Code**: `404 Not Found` </br>
    **Content**: `error as plain/text`

    **Code**: `500 Internal Server Error` </br>
    **Content**: `error as plain/text`

## Get All Article

Get all article from an user.

- **URL**: 

    /articles/{id}/{order}/

- **Method**:

    GET

- **Headers**:

    **optional**: </br>
    `Accept: text/xml` ask the server to send data as XML

- **URL Param**:

    **required**: </br>
    `id=[string]` represent an user ID

    **optional**: </br>
    `order=[desc|asc]` ask the server to order in an ascending or descending way 

- **Data Param**:

    None

- **Success Response**: 

    **Code**: `200 OK` </br>
    **Content**: 
    ```json
    [{
        "title": "My Article",
        "content": "Whatever I want to say!"
    },{
        "title": "My Other Article",
        "content": "Whatever I want to add!"
    }]
    ```

- **Error Response**: 

    **Code**: `400 Bad Request` </br>
    **Content**: `error as plain/text`

    **Code**: `404 Not Found` </br>
    **Content**: `error as plain/text`

    **Code**: `500 Internal Server Error` </br>
    **Content**: `error as plain/text`

## Delete All Article

Delete all article from an user.

- **URL**:

    /articles/{id}/

- **Method**:

    DELETE

- **URL Param**:

    **required**: </br>
    `id=[string]` represent an user ID

- **Data Param**:

    None

- **Success Response**: 

    **Code**: `200 OK` </br>
    **Content**: None

- **Error Response**: 

    **Code**: `400 Bad Request` </br>
    **Content**: `error as plain/text`

    **Code**: `404 Not Found` </br>
    **Content**: `error as plain/text`

    **Code**: `500 Internal Server Error` </br>
    **Content**: `error as plain/text`