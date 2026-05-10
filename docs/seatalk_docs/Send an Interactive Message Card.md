Send an Interactive Message Card
Request Parameter
For a comprehensive introduction to building an interactive message card, see Build a Card.

Header

Parameter

Type

Mandatory

Description

Default

Sample

Authorization

string

Yes

Obtained through the Get App Access Token API

 

Bearer c8bda0f77ef940c5bea9f23b2d7fc0d8

Content-Type

string

Yes

Content-Type

 

application/json

Body

Parameter

Type

Mandatory

Length/Size Limit

Description

Default

Sample

tag

string

Yes

20 characters

The type of the service notice message

N/A

"interactive_message"

(message_object)

object

Yes

N/A 

- The message object whose structure depends on the specified "tag" field

- For a comprehensive introduction to building an interactive message card, see Build a Card

N/A

See the request body sample below

employee_codes

[]string

Yes

Min: 1 recipient

Max: 50 recipients

A list of employee_codes that specify the recipients of this service notice message

N/A

["abcdefg", "hijklmn"]

usable_platform

string

No

N/A 

The platform(s) where the service notice message can be viewed fully and acted on (e.g., tapping a button on an interactive message card)

- "all": all platforms (mobile + desktop)

- "mobile": mobile platforms only (iOS + Android). On desktop platforms, a default message will be shown with text "[Interactive Message] This message can only be viewed on mobile devices due to the App's setting. Please check this message on SeaTalk Mobile App."

- "desktop": desktop platforms only (Desktop + Web). On mobile platforms, a default message will be shown with text "[Interactive Message] This message can only be viewed on desktop devices due to the App's setting. Please check this message on SeaTalk Desktop/Web App."

"all"

"mobile"

Request Body Sample

{
    "tag": "interactive_message",
    "interactive_message": {
        "default": {
            "elements": [
                {
                    "element_type": "title",
                    "title": {
                        "text": "Mail pending for collection"
                    }
                },
                {
                    "element_type": "description",
                    "description": {
                        "text": "You have a mail at the office lobby pending for collection. Please visit the lobby during the office hours to collect it."
                    }
                },
                {
                    "element_type": "button",
                    "button": {
                        "button_type": "redirect",
                        "text": "View details",
                        "mobile_link": {
                            "type": "web",
                            "path": "https://webApp.com/somePath"
                        },
                        "desktop_link": {
                            "type": "web",
                            "path": "https://webApp.com/somePath"
                        }
                    }
                },
                {
                    "element_type": "button",
                    "button": {
                        "button_type": "callback",
                        "text": "I have collected it",
                        "value": "collected"
                    }
                }
            ]
        },
        "zh-Hans": {
            "elements": [
                {
                    "element_type": "title",
                    "title": {
                        "text": "待取信件"
                    }
                },
                {
                    "element_type": "description",
                    "description": {
                        "text": "你有一封待取的信件，请在办公时间段前往大厅领取。"
                    }
                },
                {
                    "element_type": "button",
                    "button": {
                        "button_type": "redirect",
                        "text": "查看详情",
                        "mobile_link": {
                            "type": "web",
                            "path": "https://webApp.com/somePath"
                        },
                        "desktop_link": {
                            "type": "web",
                            "path": "https://webApp.com/somePath"
                        }
                    }
                },
                {
                    "element_type": "button",
                    "button": {
                        "button_type": "callback",
                        "text": "我已取件",
                        "value": "collected"
                    }
                }
            ]
        }
    },
    "employee_codes": [
        "abcdegf",
        "hijklmn",
        "opqrstu"
    ]
}

Copy
Response Parameter
Result Fields

Parameters

Type

Mandatory

Description

code

int

Yes

- Refer to Error Codes for explanations

- 0 if the message is sent successfully

delivery

[]object

No

The information about the delivery of the message

∟code

int

Yes

- The delivery status of the message sent to a particular user

- Refer to Error Codes

- 0 if the message is sent successfully to this user

∟employee_code

string

Yes

employee_code of the recipient

∟message_id

string

No

The unique identifier of the message if it has been successfully sent out to this user

Response Sample

{
    "code": 0,
    "delivery": [
        {
            "code": 0,
            "employee_code": "abcdefg",
            "message_id": "abcdefghijklmn"
        },
        {
            "code": 0,
            "employee_code": "hijklmn",
            "message_id": "opqrstuvwxyzab"
        },
        {
            "code": 3001,
            "employee_code": "opqrstu"
        }
    ]
}
