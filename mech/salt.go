package mech

// Salt re-rolls the bot's random choices without changing its logic: practice seeds are fixed, so different
// salts give different practice scores with the same expected play. 0 keeps the original seeding.
var Salt int64 = 0
