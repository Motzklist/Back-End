# 📚 Motzklist API Gateway Schema (v1.0)

This document defines the RESTful JSON contract for the `api-gateway` service. All endpoints are prefixed with `/api/`.

## Localization

School, grade, and equipment names are multi-language. Every public read endpoint
(`/api/schools`, `/api/grades`, `/api/equipment`, `/api/cart`, `/api/history`)
accepts an optional **`lang`** query parameter:

| `lang` | Behaviour |
| :--- | :--- |
| _absent_ or anything other than `he` | Returns the default (English) name. |
| `he` | Returns the Hebrew translation, falling back to the English name when no translation exists. |

The `name` field in each object below is already localized to the requested
`lang`; clients simply render it as-is. Translations are stored in the database
in parallel `*_he` columns (`sname_he`, `gname_he`, `ename_he`) and can be managed
through the admin endpoints, which expose an optional `nameHe` field.

## Core Data Structures

The following JSON structures are used across all endpoints:

### School Object
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string` | Unique internal identifier for the school (e.g., "1"). |
| `name` | `string` | Display name of the school (e.g., "Ben Gurion"). |

### Grade Object
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string` | Unique identifier for the grade level (e.g., "9"). |
| `name` | `string` | Display name of the grade (e.g., "9th Grade"). |

> **Note:** The Class Object has been removed from the schema as per the client's vision. All logic and data structures now depend only on school and grade.

### Equipment Object
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string` | Unique identifier for the item (e.g., "201"). |
| `name` | `string` | Display name of the equipment (e.g., "Engineering Calculator"). |
| `quantity` | `integer` | The required number of this item. |

---

## Endpoint Definitions

### 1. Get All Schools

Retrieves the initial list of schools to populate the first dropdown.

| Detail | Value |
| :--- | :--- |
| **Path** | `/api/schools` |
| **Method** | `GET` |
| **Authentication** | None (MVP) |

#### Request
* **Query Parameters:** `lang` (optional) — `he` for Hebrew names, otherwise English.
* **Body:** None.

#### Response (200 OK)
* **Body:** Array of `School` objects.
```json
[
  {
    "id": "1",
    "name": "Ben Gurion"
  },
  {
    "id": "2",
    "name": "ORT"
  }
]
```
