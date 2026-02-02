from locust import task, between
from locust.contrib.fasthttp import FastHttpUser
import random
import time
import json

class AlbumUser(FastHttpUser):
    wait_time = between(0.3, 1.0)

    @task(3)
    def get_albums(self):
        self.client.get("/albums", name="GET /albums")

    @task(1)
    def post_album(self):
        new_id = str(int(time.time() * 1000)) + str(random.randint(0, 999))
        payload = {
            "id": new_id,
            "title": "LoadTest Album",
            "artist": "Locust",
            "price": 9.99
        }

        # FastHttpUser safest: send JSON string + content-type header
        self.client.post(
            "/albums",
            data=json.dumps(payload),
            headers={"Content-Type": "application/json"},
            name="POST /albums",
        )

