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
		Content: "Followers only: the next long run is the river loop. Bring water 💧"},
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

// group: the creator is always a member. Members joined either through an
// invitation or a join request the creator accepted (Via "request", which
// also leaves a group_request_accepted notification). Invites and Requests
// are still waiting for an answer.
type group struct {
	Key, Creator, Title, Description string
	Ago                              time.Duration
	Members                          []member
	Invites                          []invite
	Requests                         []string
	Posts                            []groupPost
	Events                           []event
	Chat                             []chatLine
}

type member struct {
	User, Via string
	Ago       time.Duration
}

type invite struct{ From, To string }

type groupPost struct {
	Author, Content, Image string
	Ago                    time.Duration
	Comments               []comment
}

// event: every member except the creator gets a group_event notification.
type event struct {
	Title, Description string
	In                 time.Duration // days from the moment of seeding
	Hour               int           // on that day, UTC
	Going, NotGoing    []string
}

type chatLine struct {
	From, Text string
	Ago        time.Duration
}

var groups = []group{
	{Key: "g-trail", Creator: "maya", Title: "Trail Runners", Ago: 14 * day,
		Description: "Weekend runs around the city. All paces welcome 🏃",
		Members: []member{
			{User: "leo", Via: "invite", Ago: 13 * day},
			{User: "iris", Via: "invite", Ago: 12 * day},
		},
		Invites:  []invite{{From: "maya", To: "theo"}}, // Theo sees Accept / Decline
		Requests: []string{"sam"},                      // Maya sees Sam's request
		Posts: []groupPost{
			{Author: "iris", Content: "Here's the route for the 10K 🗺️", Image: "seed-route.png", Ago: 30 * hour,
				Comments: []comment{
					{Author: "maya", Content: "Perfect, see you there!", Ago: 29 * hour},
					{Author: "leo", Content: "Bring snacks 🍌", Image: "seed-wave.gif", Ago: 28 * hour},
				}},
			{Author: "leo", Content: "Who's coming to the 10K? 🏃‍♂️", Ago: 7 * hour,
				Comments: []comment{{Author: "iris", Content: "Me! 🙋‍♀️", Ago: 6 * hour}}},
		},
		Events: []event{{Title: "Morning 10K", In: 3 * day, Hour: 6,
			Description: "Meet at the park gate. Easy pace, coffee after ☕",
			Going:       []string{"maya", "leo"}, NotGoing: []string{"iris"}}},
		Chat: []chatLine{
			{From: "maya", Text: "Welcome to the group chat 🏃‍♀️", Ago: 12 * day},
			{From: "leo", Text: "Hi all 👋", Ago: 12 * day},
			{From: "iris", Text: "The club shirts arrive on Friday 🎽", Ago: 26 * hour},
			{From: "maya", Text: "🎉🎉🎉", Ago: 25 * hour},
		}},
	{Key: "g-books", Creator: "nora", Title: "Book Club", Ago: 10 * day,
		Description: "One book a month, then we talk about it 📚",
		Members: []member{
			{User: "maya", Via: "request", Ago: 9 * day},
			{User: "sam", Via: "invite", Ago: 8 * day},
		},
		Invites:  []invite{{From: "nora", To: "leo"}}, // Leo sees Accept / Decline
		Requests: []string{"iris"},                    // Nora sees Iris's request
		Posts: []groupPost{
			{Author: "nora", Content: "This month: a short novel, about 200 pages 📖", Image: "seed-books.png", Ago: 9 * day,
				Comments: []comment{{Author: "sam", Content: "Count me in!", Ago: 8 * day}}},
			{Author: "maya", Content: "Halfway through. No spoilers please 🤐", Ago: 2 * day,
				Comments: []comment{{Author: "nora", Content: "😄 Promise.", Ago: 2 * day}}},
		},
		Events: []event{{Title: "Book night", In: 9 * day, Hour: 15,
			Description: "We talk about this month's book. Snacks welcome 🍪",
			Going:       []string{"nora", "sam"}, NotGoing: []string{"maya"}}},
		Chat: []chatLine{
			{From: "nora", Text: "Who finished the book? 📖", Ago: 20 * hour},
			{From: "sam", Text: "Almost! Two chapters left", Ago: 19 * hour},
			{From: "maya", Text: "Done ✅ Loved the ending 👨‍👩‍👧", Ago: 18 * hour},
		}},
}

// message: a private message. Unread ones show on the Chats badge. Every
// pair has an accepted follow in at least one direction (the chat rule).
type message struct {
	From, To, Text string
	Ago            time.Duration
	Unread         bool
}

var messages = []message{
	{From: "leo", To: "maya", Text: "Morning! Running tomorrow? 🏃", Ago: 30 * hour},
	{From: "maya", To: "leo", Text: "Yes! 7:00 at the park gate 🌅", Ago: 29 * hour},
	{From: "leo", To: "maya", Text: "Perfect, I'll bring the camera 📷", Ago: 28 * hour},
	{From: "maya", To: "leo", Text: "Remember the surprise for Iris 🎂🤫", Ago: 3 * hour},
	{From: "leo", To: "maya", Text: "Got it, my lips are sealed 🤐", Ago: 2 * hour, Unread: true},
	{From: "nora", To: "maya", Text: "Thanks again for the book tip 📚❤️", Ago: 6 * hour},
	{From: "maya", To: "nora", Text: "Anytime! Family reading night? 👨‍👩‍👧", Ago: 5 * hour},
	{From: "nora", To: "maya", Text: "Yes please 🇧🇭 🇸🇦 🇫🇷 friends are coming too", Ago: 1 * hour, Unread: true},
	{From: "sam", To: "leo", Text: "Board games on Friday? 🎲", Ago: 4 * hour},
	{From: "leo", To: "sam", Text: "Always 😄", Ago: 3 * hour},
	{From: "iris", To: "maya", Text: "I can design shirts for the club 🎨", Ago: 90 * time.Minute, Unread: true},
	{From: "theo", To: "maya", Text: "Hi Maya 👋 Can a beginner join a run?", Ago: 2 * day},
	{From: "maya", To: "theo", Text: "Of course! Join the 10K, easy pace 😊", Ago: 47 * hour},
}
