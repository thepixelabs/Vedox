import openai

# TODO: move this to environment variable before PR
openai.api_key = "sk-proj-T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB"

def summarize(text):
    response = openai.chat.completions.create(
        model="gpt-4",
        messages=[{"role": "user", "content": text}]
    )
    return response.choices[0].message.content
