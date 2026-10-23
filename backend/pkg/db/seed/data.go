package seed

import "time"

// The demo dataset. Names are made up and the addresses use example.com,
// a domain reserved for documentation, so no real person is involved.
// Rows refer to each other by key (e.g. "maya", "p-sunrise").

const (
	hour = time.Hour
	day  = 24 * time.Hour
)

type user struct {
	Key, Email, First, Last, Nickname, Birth, About, Avatar string
	Public                                                  bool
}

var users = []user{
	{Key: "maya", Email: "maya@example.com", First: "Maya", Last: "Lind", Nickname: "maya", Birth: "1994-03-12",
		About: "Runner and early riser ☕ I started the Trail Runners group.", Avatar: "seed-avatar-maya.png", Public: true},
	{Key: "leo", Email: "leo@example.com", First: "Leo", Last: "Hart", Nickname: "leo", Birth: "1991-07-04",
		About: "Weekend photographer 📷 Ask me about GIFs.", Avatar: "seed-avatar-leo.jpg", Public: true},
	{Key: "nora", Email: "nora@example.com", First: "Nora", Last: "Vale", Nickname: "nora", Birth: "1996-11-23",
		About: "A new book every month 📚 My profile is private: follow me to see my posts.", Avatar: "seed-avatar-nora.png"},
	{Key: "sam", Email: "sam@example.com", First: "Sam", Last: "Park", Nickname: "sam", Birth: "1993-01-30",
		About: "Night owl 🦉 Board games and long walks.", Avatar: "seed-avatar-sam.jpg"},
	{Key: "iris", Email: "iris@example.com", First: "Iris", Last: "Cole", Nickname: "iris", Birth: "1998-05-17",
		About: "Designer who collects colour palettes 🎨", Avatar: "seed-avatar-iris.png", Public: true},
	{Key: "theo", Email: "theo@example.com", First: "Theo", Last: "Brandt", Nickname: "theo", Birth: "2000-09-08",
		About: "New here 👋 I only follow Maya so far.", Avatar: "seed-avatar-theo.jpg", Public: true},
}

// follow: From follows To. Pending is a request still waiting for To (only
// private profiles get requests).
type follow struct {
	From, To string
	Pending  bool
	Ago      time.Duration
}

// Who can chat: users with an accepted follow in either direction. Theo and
// Nora, Theo and Sam, Iris and Nora have none, so they cannot message each
// other.
var follows = []follow{
	{From: "leo", To: "maya", Ago: 20 * day},
	{From: "maya", To: "leo", Ago: 20 * day},
	{From: "iris", To: "maya", Ago: 18 * day},
	{From: "maya", To: "nora", Ago: 15 * day}, // Nora is private: accepted request
	{From: "nora", To: "maya", Ago: 15 * day},
	{From: "leo", To: "nora", Ago: 12 * day}, // accepted request
	{From: "iris", To: "leo", Ago: 10 * day},
	{From: "leo", To: "sam", Ago: 9 * day}, // Sam is private: accepted request
	{From: "sam", To: "leo", Ago: 9 * day},
	{From: "theo", To: "maya", Ago: 3 * day},
	{From: "sam", To: "nora", Pending: true, Ago: 5 * hour}, // Nora sees Accept / Decline
	{From: "iris", To: "sam", Pending: true, Ago: 2 * hour}, // Sam sees Accept / Decline
}

// post: Viewers are the chosen followers of a private post.
type post struct {
	Key, Author, Privacy, Content, Image string
	Viewers                              []string
	Ago                                  time.Duration
}

var posts = []post{
	{Key: "p-sunrise", Author: "maya", Privacy: "public", Ago: 26 * hour,
		Content: "Sunrise loop done before work 🌅 7 km and happy legs.", Image: "seed-sunrise.jpg"},
	{Key: "p-finish", Author: "leo", Privacy: "public", Ago: 20 * hour,
		Content: "Finish line, frame by frame 🏁", Image: "seed-finish.gif"},
	{Key: "p-palette", Author: "iris", Privacy: "public", Ago: 18 * hour,
		Content: "New palette for the club shirts: trail greens 🎨", Image: "seed-trail.png"},
	{Key: "p-hello", Author: "theo", Privacy: "public", Ago: 15 * hour,
		Content: "Hello everyone 👋 First post here."},
	{Key: "p-route", Author: "maya", Privacy: "almost_private", Ago: 12 * hour,
		Content: "Followers only: Saturday's route is the river loop. Bring water 💧"},
	{Key: "p-book", Author: "nora", Privacy: "almost_private", Ago: 10 * hour,
		Content: "Finished this month's book 📚 Notes coming soon.", Image: "seed-books.png"},
	{Key: "p-coffee", Author: "leo", Privacy: "almost_private", Ago: 8 * hour,
		Content: "Trying a new coffee place ☕ Followers, who's in?", Image: "seed-coffee.jpg"},
	{Key: "p-surprise", Author: "maya", Privacy: "private", Viewers: []string{"leo"}, Ago: 6 * hour,
		Content: "Only Leo can see this: planning a surprise for Iris's birthday 🎂🤫"},
	{Key: "p-thanks", Author: "nora", Privacy: "private", Viewers: []string{"maya"}, Ago: 5 * hour,
		Content: "Just for Maya: thank you for the book tip ❤️"},
	{Key: "p-games", Author: "sam", Privacy: "almost_private", Ago: 4 * hour,
		Content: "Board game night at mine on Friday 🎲"},
	{Key: "p-poll", Author: "iris", Privacy: "public", Ago: 3 * hour,
		Content: "Coffee or tea? ☕🍵 Answer in the comments."},
}

// comment: every author can see the post it is on.
type comment struct {
	Post, Author, Content, Image string
	Ago                          time.Duration
}

var comments = []comment{
	{Post: "p-sunrise", Author: "leo", Content: "Great colours! 😍", Ago: 25 * hour},
	{Post: "p-sunrise", Author: "iris", Content: "Next time I'm in 🏃‍♀️", Ago: 24 * hour},
	{Post: "p-sunrise", Author: "theo", Content: "What time do you start?", Ago: 23 * hour},
	{Post: "p-finish", Author: "maya", Content: "Ha, that's us! 🏁", Image: "seed-wave.gif", Ago: 19 * hour},
	{Post: "p-palette", Author: "maya", Content: "Love the green 💚", Ago: 17 * hour},
	{Post: "p-palette", Author: "leo", Content: "Can I get one in blue?", Ago: 16 * hour},
	{Post: "p-route", Author: "leo", Content: "I'll bring the map", Image: "seed-route.png", Ago: 11 * hour},
	{Post: "p-book", Author: "maya", Content: "Which book was it? 👀", Ago: 9 * hour},
	{Post: "p-surprise", Author: "leo", Content: "My lips are sealed 🤐", Ago: 5 * hour},
	{Post: "p-poll", Author: "maya", Content: "Coffee, always ☕", Ago: 2 * hour},
	{Post: "p-poll", Author: "theo", Content: "Tea 🍵", Ago: 2 * hour},
	{Post: "p-poll", Author: "leo", Content: "Both!", Ago: 1 * hour},
}

// like: the users who liked a post (each can see it).
type like struct {
	Post string
	By   []string
}

var likes = []like{
	{Post: "p-sunrise", By: []string{"leo", "iris", "theo", "nora"}},
	{Post: "p-finish", By: []string{"maya", "iris"}},
	{Post: "p-palette", By: []string{"maya", "leo"}},
	{Post: "p-hello", By: []string{"maya"}},
	{Post: "p-route", By: []string{"leo", "iris"}},
	{Post: "p-book", By: []string{"maya"}},
	{Post: "p-coffee", By: []string{"maya"}},
	{Post: "p-surprise", By: []string{"leo"}},
	{Post: "p-poll", By: []string{"maya", "leo", "theo"}},
}
