Set Typing Status in Group Chat
Typing status can only be triggered by bots in SeaTalk v3.55 or later

API Description
Set the "Typing..." indicator in a private chat between your bot and its user. This API should be used when your bot receives an event from a user in a private chat and needs an extended amount of time to generate a response. On successful call, the typing indicator is displayed in the chatroom for 4s.

Prerequisite: 

This API can only be called within 30s of receiving any of the following events: 
Interactive Message Click 
Bot Added to Group Chat
New Mentioned Message From Group Chat
The group_id + thread_id from the event received must match the group_id + thread_id from the event received
Request Method: POST

End Point: https://openapi.seatalk.io/messaging/v2/group_chat_typing

Rate Limit: 300/min

Request Parameters
Header

Parameter	Type 	Mandatory	Description	Default	Sample
Authorization	string	Yes	From Get App Access Token	N/A	Bearer c8bda0f77ef940c5bea9f23b2d7fc0d8
Content-Type

string

Yes

Request header format

N/A

application/json

Body

Parameter	Type	Mandatory	Description	Default	Length Limit	Sample
group_id	string	Yes	The group chat ID	
N/A

N/A

“abcdef”

thread_id	string	No	
The thread ID. Provide thread_id to trigger typing status in a thread. 

To start typing in an unthreaded root message, define thread_id as the message_id of the root message.
The root message has to be sent within the past 7 days.
N/A	N/A	“abcdef”
Request Sample

{
    "group_id": "MTI2OTA1OTM5OTk0",
    "thread_id": "rSAS8xiQOrLdTuXkvqrsScRVALBcU6vmZPufqVlC2CWyV9hVZlX9HITB"
}

Copy
Response Parameters
Body

Parameter	Type	Description	Sample
code	int	Refer to Error Code for explanations	0
Error Codes 
Code	Description	Resolution
4014	No event received from chat	Typing status could not be triggered in this chat as no event was received by the app within the last 30s.
7003	Group chat too large 	Typing status could not be triggered in this group as it has more than 200 members