package categorizer

// Categories defines all supported expense categories and subcategories.
var Categories = map[string][]string{
	"Food & Dining":      {"Restaurant", "Food Delivery", "Cafe", "Fast Food", "Bar & Pub"},
	"Groceries":          {"Supermarket", "Online Grocery", "Quick Commerce"},
	"Shopping":           {"E-Commerce", "Fashion", "Electronics", "Eyewear", "Beauty", "Home & Furniture"},
	"Travel":             {"Flight", "Hotel", "Train", "Bus", "Booking"},
	"Transport":          {"Cab", "Auto", "Metro", "Parking", "Toll"},
	"Fuel":               {"Petrol", "Diesel", "CNG", "EV Charging"},
	"Entertainment":      {"Movies", "Streaming", "Music", "Events", "Gaming"},
	"Health & Medical":   {"Pharmacy", "Doctor", "Lab Tests", "Hospital", "Fitness"},
	"Bills & Utilities":  {"Electricity", "Internet", "Mobile", "Water", "Gas", "DTH"},
	"Insurance":          {"Life", "Health", "Motor", "General"},
	"EMI & Loans":        {"Personal Loan", "Home Loan", "Car Loan", "Credit Card EMI"},
	"Education":          {"Online Course", "Books", "School", "College"},
	"Subscriptions":      {"Entertainment", "Software", "Cloud", "News"},
	"Government & Tax":   {"Income Tax", "GST", "Passport", "Challan"},
	"Rent":               {},
	"Payments & Transfers": {"Wallet", "UPI", "Bank Transfer"},
	"Miscellaneous":      {},
}

// AllCategories returns a flat list of category names.
func AllCategories() []string {
	cats := make([]string, 0, len(Categories))
	for c := range Categories {
		cats = append(cats, c)
	}
	return cats
}
