package categorizer

// Rule maps a merchant pattern to category information.
type Rule struct {
	Pattern     string
	Merchant    string
	Company     string
	Category    string
	SubCategory string
}

// BuiltinRules contains 100+ Indian merchant categorization rules.
var BuiltinRules = []Rule{
	// Food & Dining - Food Delivery
	{Pattern: "SWIGGY", Merchant: "Swiggy", Company: "Bundl Technologies", Category: "Food & Dining", SubCategory: "Food Delivery"},
	{Pattern: "BUNDL TECHNOLOGIES", Merchant: "Swiggy", Company: "Bundl Technologies", Category: "Food & Dining", SubCategory: "Food Delivery"},
	{Pattern: "ZOMATO", Merchant: "Zomato", Company: "Zomato Ltd", Category: "Food & Dining", SubCategory: "Food Delivery"},
	{Pattern: "ZOMATO LIMITED", Merchant: "Zomato", Company: "Zomato Ltd", Category: "Food & Dining", SubCategory: "Food Delivery"},
	{Pattern: "DUNZO", Merchant: "Dunzo", Company: "Dunzo Digital", Category: "Food & Dining", SubCategory: "Food Delivery"},

	// Food & Dining - Restaurants & Fast Food
	{Pattern: "DOMINOS", Merchant: "Domino's", Company: "Jubilant FoodWorks", Category: "Food & Dining", SubCategory: "Fast Food"},
	{Pattern: "MCDONALD", Merchant: "McDonald's", Company: "Westlife Foodworld", Category: "Food & Dining", SubCategory: "Fast Food"},
	{Pattern: "KFC", Merchant: "KFC", Company: "Yum! Brands", Category: "Food & Dining", SubCategory: "Fast Food"},
	{Pattern: "BURGER KING", Merchant: "Burger King", Company: "Restaurant Brands Asia", Category: "Food & Dining", SubCategory: "Fast Food"},
	{Pattern: "PIZZA HUT", Merchant: "Pizza Hut", Company: "Yum! Brands", Category: "Food & Dining", SubCategory: "Fast Food"},
	{Pattern: "SUBWAY", Merchant: "Subway", Company: "Subway", Category: "Food & Dining", SubCategory: "Fast Food"},
	{Pattern: "HALDIRAM", Merchant: "Haldiram's", Company: "Haldiram's", Category: "Food & Dining", SubCategory: "Restaurant"},
	{Pattern: "BARBEQUE NATION", Merchant: "Barbeque Nation", Company: "Barbeque Nation", Category: "Food & Dining", SubCategory: "Restaurant"},

	// Food & Dining - Cafe
	{Pattern: "STARBUCKS", Merchant: "Starbucks", Company: "Tata Starbucks", Category: "Food & Dining", SubCategory: "Cafe"},
	{Pattern: "CHAAYOS", Merchant: "Chaayos", Company: "Sunshine Teahouse", Category: "Food & Dining", SubCategory: "Cafe"},
	{Pattern: "BLUE TOKAI", Merchant: "Blue Tokai", Company: "Blue Tokai Coffee", Category: "Food & Dining", SubCategory: "Cafe"},
	{Pattern: "THIRD WAVE", Merchant: "Third Wave Coffee", Company: "Third Wave Coffee", Category: "Food & Dining", SubCategory: "Cafe"},
	{Pattern: "CAFE COFFEE DAY", Merchant: "Cafe Coffee Day", Company: "Coffee Day Enterprises", Category: "Food & Dining", SubCategory: "Cafe"},
	{Pattern: "CCD", Merchant: "Cafe Coffee Day", Company: "Coffee Day Enterprises", Category: "Food & Dining", SubCategory: "Cafe"},

	// Groceries - Online
	{Pattern: "BIGBASKET", Merchant: "BigBasket", Company: "Supermarket Grocery Supplies", Category: "Groceries", SubCategory: "Online Grocery"},
	{Pattern: "BBNOW", Merchant: "BigBasket", Company: "Supermarket Grocery Supplies", Category: "Groceries", SubCategory: "Online Grocery"},
	{Pattern: "BLINKIT", Merchant: "Blinkit", Company: "Zomato (Blinkit)", Category: "Groceries", SubCategory: "Quick Commerce"},
	{Pattern: "GROFERS", Merchant: "Blinkit", Company: "Zomato (Blinkit)", Category: "Groceries", SubCategory: "Quick Commerce"},
	{Pattern: "ZEPTO", Merchant: "Zepto", Company: "KiranaKart Technologies", Category: "Groceries", SubCategory: "Quick Commerce"},
	{Pattern: "INSTAMART", Merchant: "Swiggy Instamart", Company: "Bundl Technologies", Category: "Groceries", SubCategory: "Quick Commerce"},
	{Pattern: "JIOMART", Merchant: "JioMart", Company: "Reliance Retail", Category: "Groceries", SubCategory: "Online Grocery"},

	// Groceries - Supermarket
	{Pattern: "DMART", Merchant: "DMart", Company: "Avenue Supermarts", Category: "Groceries", SubCategory: "Supermarket"},
	{Pattern: "AVENUE SUPERMARTS", Merchant: "DMart", Company: "Avenue Supermarts", Category: "Groceries", SubCategory: "Supermarket"},
	{Pattern: "MORE RETAIL", Merchant: "More", Company: "More Retail", Category: "Groceries", SubCategory: "Supermarket"},
	{Pattern: "SPENCER", Merchant: "Spencer's", Company: "Spencer's Retail", Category: "Groceries", SubCategory: "Supermarket"},
	{Pattern: "STAR BAZAAR", Merchant: "Star Bazaar", Company: "Trent Hypermarket", Category: "Groceries", SubCategory: "Supermarket"},
	{Pattern: "RELIANCE FRESH", Merchant: "Reliance Fresh", Company: "Reliance Retail", Category: "Groceries", SubCategory: "Supermarket"},
	{Pattern: "NATURE'S BASKET", Merchant: "Nature's Basket", Company: "Spencer's Retail", Category: "Groceries", SubCategory: "Supermarket"},

	// Shopping - E-Commerce
	{Pattern: "AMAZON", Merchant: "Amazon", Company: "Amazon India", Category: "Shopping", SubCategory: "E-Commerce"},
	{Pattern: "AMZN", Merchant: "Amazon", Company: "Amazon India", Category: "Shopping", SubCategory: "E-Commerce"},
	{Pattern: "APAY", Merchant: "Amazon Pay", Company: "Amazon India", Category: "Shopping", SubCategory: "E-Commerce"},
	{Pattern: "FLIPKART", Merchant: "Flipkart", Company: "Flipkart Internet", Category: "Shopping", SubCategory: "E-Commerce"},
	{Pattern: "FLIPKART INTERNET", Merchant: "Flipkart", Company: "Flipkart Internet", Category: "Shopping", SubCategory: "E-Commerce"},
	{Pattern: "MEESHO", Merchant: "Meesho", Company: "Fashnear Technologies", Category: "Shopping", SubCategory: "E-Commerce"},
	{Pattern: "SNAPDEAL", Merchant: "Snapdeal", Company: "Snapdeal", Category: "Shopping", SubCategory: "E-Commerce"},

	// Shopping - Fashion
	{Pattern: "MYNTRA", Merchant: "Myntra", Company: "Flipkart (Myntra)", Category: "Shopping", SubCategory: "Fashion"},
	{Pattern: "AJIO", Merchant: "AJIO", Company: "Reliance Retail", Category: "Shopping", SubCategory: "Fashion"},
	{Pattern: "NYKAA", Merchant: "Nykaa", Company: "FSN E-Commerce", Category: "Shopping", SubCategory: "Beauty"},
	{Pattern: "LENSKART", Merchant: "Lenskart", Company: "Lenskart Solutions", Category: "Shopping", SubCategory: "Eyewear"},
	{Pattern: "TITAN", Merchant: "Titan", Company: "Titan Company", Category: "Shopping", SubCategory: "Fashion"},
	{Pattern: "TANISHQ", Merchant: "Tanishq", Company: "Titan Company", Category: "Shopping", SubCategory: "Jewellery"},
	{Pattern: "ZARA", Merchant: "Zara", Company: "Inditex", Category: "Shopping", SubCategory: "Fashion"},
	{Pattern: "H&M", Merchant: "H&M", Company: "H&M", Category: "Shopping", SubCategory: "Fashion"},
	{Pattern: "UNIQLO", Merchant: "Uniqlo", Company: "Fast Retailing", Category: "Shopping", SubCategory: "Fashion"},
	{Pattern: "WESTSIDE", Merchant: "Westside", Company: "Trent Ltd", Category: "Shopping", SubCategory: "Fashion"},
	{Pattern: "PANTALOONS", Merchant: "Pantaloons", Company: "Aditya Birla Fashion", Category: "Shopping", SubCategory: "Fashion"},

	// Shopping - Electronics
	{Pattern: "CROMA", Merchant: "Croma", Company: "Infiniti Retail", Category: "Shopping", SubCategory: "Electronics"},
	{Pattern: "RELIANCE DIGITAL", Merchant: "Reliance Digital", Company: "Reliance Retail", Category: "Shopping", SubCategory: "Electronics"},
	{Pattern: "APPLE.COM", Merchant: "Apple", Company: "Apple Inc", Category: "Shopping", SubCategory: "Electronics"},
	{Pattern: "APPLE STORE", Merchant: "Apple", Company: "Apple Inc", Category: "Shopping", SubCategory: "Electronics"},

	// Shopping - Home & Furniture
	{Pattern: "IKEA", Merchant: "IKEA", Company: "Ingka Group", Category: "Shopping", SubCategory: "Home & Furniture"},
	{Pattern: "PEPPERFRY", Merchant: "Pepperfry", Company: "Trendsutra", Category: "Shopping", SubCategory: "Home & Furniture"},
	{Pattern: "URBAN LADDER", Merchant: "Urban Ladder", Company: "Reliance Retail", Category: "Shopping", SubCategory: "Home & Furniture"},

	// Travel - Booking
	{Pattern: "MAKEMYTRIP", Merchant: "MakeMyTrip", Company: "MakeMyTrip Ltd", Category: "Travel", SubCategory: "Booking"},
	{Pattern: "MMT", Merchant: "MakeMyTrip", Company: "MakeMyTrip Ltd", Category: "Travel", SubCategory: "Booking"},
	{Pattern: "GOIBIBO", Merchant: "Goibibo", Company: "MakeMyTrip Ltd", Category: "Travel", SubCategory: "Booking"},
	{Pattern: "CLEARTRIP", Merchant: "Cleartrip", Company: "Flipkart (Cleartrip)", Category: "Travel", SubCategory: "Booking"},
	{Pattern: "EASEMYTRIP", Merchant: "EaseMyTrip", Company: "Easy Trip Planners", Category: "Travel", SubCategory: "Booking"},
	{Pattern: "IXIGO", Merchant: "ixigo", Company: "Le Travenues Technology", Category: "Travel", SubCategory: "Booking"},
	{Pattern: "YATRA", Merchant: "Yatra", Company: "Yatra Online", Category: "Travel", SubCategory: "Booking"},
	{Pattern: "BOOKING.COM", Merchant: "Booking.com", Company: "Booking Holdings", Category: "Travel", SubCategory: "Hotel"},
	{Pattern: "AIRBNB", Merchant: "Airbnb", Company: "Airbnb Inc", Category: "Travel", SubCategory: "Hotel"},
	{Pattern: "OYO", Merchant: "OYO", Company: "Oravel Stays", Category: "Travel", SubCategory: "Hotel"},

	// Travel - Flight
	{Pattern: "INDIGO", Merchant: "IndiGo", Company: "InterGlobe Aviation", Category: "Travel", SubCategory: "Flight"},
	{Pattern: "INTERGLOBE", Merchant: "IndiGo", Company: "InterGlobe Aviation", Category: "Travel", SubCategory: "Flight"},
	{Pattern: "AIR INDIA", Merchant: "Air India", Company: "Air India Ltd", Category: "Travel", SubCategory: "Flight"},
	{Pattern: "AIRINDIA", Merchant: "Air India", Company: "Air India Ltd", Category: "Travel", SubCategory: "Flight"},
	{Pattern: "SPICEJET", Merchant: "SpiceJet", Company: "SpiceJet Ltd", Category: "Travel", SubCategory: "Flight"},
	{Pattern: "VISTARA", Merchant: "Vistara", Company: "Air India (Vistara)", Category: "Travel", SubCategory: "Flight"},
	{Pattern: "AKASA", Merchant: "Akasa Air", Company: "SNV Aviation", Category: "Travel", SubCategory: "Flight"},

	// Travel - Train
	{Pattern: "IRCTC", Merchant: "IRCTC", Company: "IRCTC", Category: "Travel", SubCategory: "Train"},

	// Transport - Cab
	{Pattern: "UBER", Merchant: "Uber", Company: "Uber India", Category: "Transport", SubCategory: "Cab"},
	{Pattern: "UBER INDIA", Merchant: "Uber", Company: "Uber India", Category: "Transport", SubCategory: "Cab"},
	{Pattern: "OLA", Merchant: "Ola", Company: "ANI Technologies", Category: "Transport", SubCategory: "Cab"},
	{Pattern: "ANI TECHNOLOGIES", Merchant: "Ola", Company: "ANI Technologies", Category: "Transport", SubCategory: "Cab"},
	{Pattern: "RAPIDO", Merchant: "Rapido", Company: "Roppen Transportation", Category: "Transport", SubCategory: "Cab"},
	{Pattern: "NAMMA YATRI", Merchant: "Namma Yatri", Company: "Juspay Technologies", Category: "Transport", SubCategory: "Cab"},

	// Transport - Metro/Parking
	{Pattern: "DMRC", Merchant: "Delhi Metro", Company: "DMRC", Category: "Transport", SubCategory: "Metro"},
	{Pattern: "FASTAG", Merchant: "FASTag", Company: "NHAI", Category: "Transport", SubCategory: "Toll"},

	// Fuel
	{Pattern: "HPCL", Merchant: "HPCL", Company: "Hindustan Petroleum", Category: "Fuel", SubCategory: "Petrol"},
	{Pattern: "BPCL", Merchant: "BPCL", Company: "Bharat Petroleum", Category: "Fuel", SubCategory: "Petrol"},
	{Pattern: "INDIAN OIL", Merchant: "Indian Oil", Company: "Indian Oil Corporation", Category: "Fuel", SubCategory: "Petrol"},
	{Pattern: "IOCL", Merchant: "Indian Oil", Company: "Indian Oil Corporation", Category: "Fuel", SubCategory: "Petrol"},
	{Pattern: "RELIANCE PETRO", Merchant: "Reliance Petrol", Company: "Reliance BP", Category: "Fuel", SubCategory: "Petrol"},
	{Pattern: "SHELL", Merchant: "Shell", Company: "Shell India", Category: "Fuel", SubCategory: "Petrol"},

	// Entertainment - Streaming
	{Pattern: "NETFLIX", Merchant: "Netflix", Company: "Netflix Inc", Category: "Entertainment", SubCategory: "Streaming"},
	{Pattern: "HOTSTAR", Merchant: "Disney+ Hotstar", Company: "Star India", Category: "Entertainment", SubCategory: "Streaming"},
	{Pattern: "DISNEY+", Merchant: "Disney+ Hotstar", Company: "Star India", Category: "Entertainment", SubCategory: "Streaming"},
	{Pattern: "PRIME VIDEO", Merchant: "Amazon Prime Video", Company: "Amazon India", Category: "Entertainment", SubCategory: "Streaming"},
	{Pattern: "SONYLIV", Merchant: "SonyLIV", Company: "Sony Pictures Networks", Category: "Entertainment", SubCategory: "Streaming"},
	{Pattern: "ZEE5", Merchant: "ZEE5", Company: "Zee Entertainment", Category: "Entertainment", SubCategory: "Streaming"},
	{Pattern: "JIOCINEMA", Merchant: "JioCinema", Company: "Viacom18", Category: "Entertainment", SubCategory: "Streaming"},
	{Pattern: "YOUTUBE PREMIUM", Merchant: "YouTube Premium", Company: "Google", Category: "Entertainment", SubCategory: "Streaming"},

	// Entertainment - Music
	{Pattern: "SPOTIFY", Merchant: "Spotify", Company: "Spotify AB", Category: "Entertainment", SubCategory: "Music"},
	{Pattern: "GAANA", Merchant: "Gaana", Company: "Times Internet", Category: "Entertainment", SubCategory: "Music"},
	{Pattern: "JIOSAAVN", Merchant: "JioSaavn", Company: "Reliance (Saavn)", Category: "Entertainment", SubCategory: "Music"},
	{Pattern: "APPLE MUSIC", Merchant: "Apple Music", Company: "Apple Inc", Category: "Entertainment", SubCategory: "Music"},

	// Entertainment - Movies/Events
	{Pattern: "BOOKMYSHOW", Merchant: "BookMyShow", Company: "Big Tree Entertainment", Category: "Entertainment", SubCategory: "Movies"},
	{Pattern: "PVR", Merchant: "PVR", Company: "PVR INOX Ltd", Category: "Entertainment", SubCategory: "Movies"},
	{Pattern: "INOX", Merchant: "INOX", Company: "PVR INOX Ltd", Category: "Entertainment", SubCategory: "Movies"},
	{Pattern: "CINEPOLIS", Merchant: "Cinepolis", Company: "Cinepolis India", Category: "Entertainment", SubCategory: "Movies"},

	// Health & Medical
	{Pattern: "APOLLO", Merchant: "Apollo", Company: "Apollo Hospitals", Category: "Health & Medical", SubCategory: "Pharmacy"},
	{Pattern: "PHARMEASY", Merchant: "PharmEasy", Company: "API Holdings", Category: "Health & Medical", SubCategory: "Pharmacy"},
	{Pattern: "1MG", Merchant: "1mg", Company: "Tata 1mg", Category: "Health & Medical", SubCategory: "Pharmacy"},
	{Pattern: "NETMEDS", Merchant: "Netmeds", Company: "Reliance (Netmeds)", Category: "Health & Medical", SubCategory: "Pharmacy"},
	{Pattern: "MEDPLUS", Merchant: "MedPlus", Company: "MedPlus Health", Category: "Health & Medical", SubCategory: "Pharmacy"},
	{Pattern: "PRACTO", Merchant: "Practo", Company: "Practo Technologies", Category: "Health & Medical", SubCategory: "Doctor"},
	{Pattern: "CULTFIT", Merchant: "Cult.fit", Company: "Cure.fit", Category: "Health & Medical", SubCategory: "Fitness"},
	{Pattern: "CULT.FIT", Merchant: "Cult.fit", Company: "Cure.fit", Category: "Health & Medical", SubCategory: "Fitness"},
	{Pattern: "DR LAL", Merchant: "Dr Lal PathLabs", Company: "Dr Lal PathLabs", Category: "Health & Medical", SubCategory: "Lab Tests"},
	{Pattern: "THYROCARE", Merchant: "Thyrocare", Company: "Thyrocare Technologies", Category: "Health & Medical", SubCategory: "Lab Tests"},

	// Bills & Utilities
	{Pattern: "TATA POWER", Merchant: "Tata Power", Company: "Tata Power", Category: "Bills & Utilities", SubCategory: "Electricity"},
	{Pattern: "ADANI ELECTRICITY", Merchant: "Adani Electricity", Company: "Adani Electricity", Category: "Bills & Utilities", SubCategory: "Electricity"},
	{Pattern: "BESCOM", Merchant: "BESCOM", Company: "BESCOM", Category: "Bills & Utilities", SubCategory: "Electricity"},
	{Pattern: "JIO", Merchant: "Jio", Company: "Reliance Jio", Category: "Bills & Utilities", SubCategory: "Mobile"},
	{Pattern: "AIRTEL", Merchant: "Airtel", Company: "Bharti Airtel", Category: "Bills & Utilities", SubCategory: "Mobile"},
	{Pattern: "VODAFONE", Merchant: "Vi", Company: "Vodafone Idea", Category: "Bills & Utilities", SubCategory: "Mobile"},
	{Pattern: "ACT FIBERNET", Merchant: "ACT Fibernet", Company: "Atria Convergence", Category: "Bills & Utilities", SubCategory: "Internet"},
	{Pattern: "HATHWAY", Merchant: "Hathway", Company: "Hathway Cable", Category: "Bills & Utilities", SubCategory: "Internet"},
	{Pattern: "TATA PLAY", Merchant: "Tata Play", Company: "Tata Play", Category: "Bills & Utilities", SubCategory: "DTH"},
	{Pattern: "DISH TV", Merchant: "Dish TV", Company: "Dish TV India", Category: "Bills & Utilities", SubCategory: "DTH"},

	// Subscriptions
	{Pattern: "GOOGLE *", Merchant: "Google", Company: "Google", Category: "Subscriptions", SubCategory: "Software"},
	{Pattern: "MICROSOFT", Merchant: "Microsoft", Company: "Microsoft", Category: "Subscriptions", SubCategory: "Software"},
	{Pattern: "ADOBE", Merchant: "Adobe", Company: "Adobe Inc", Category: "Subscriptions", SubCategory: "Software"},
	{Pattern: "NOTION", Merchant: "Notion", Company: "Notion Labs", Category: "Subscriptions", SubCategory: "Software"},
	{Pattern: "CHATGPT", Merchant: "ChatGPT", Company: "OpenAI", Category: "Subscriptions", SubCategory: "Software"},
	{Pattern: "OPENAI", Merchant: "OpenAI", Company: "OpenAI", Category: "Subscriptions", SubCategory: "Software"},

	// Insurance
	{Pattern: "LIC", Merchant: "LIC", Company: "Life Insurance Corporation", Category: "Insurance", SubCategory: "Life"},
	{Pattern: "ICICI LOMBARD", Merchant: "ICICI Lombard", Company: "ICICI Lombard", Category: "Insurance", SubCategory: "General"},
	{Pattern: "HDFC ERGO", Merchant: "HDFC Ergo", Company: "HDFC Ergo", Category: "Insurance", SubCategory: "General"},
	{Pattern: "STAR HEALTH", Merchant: "Star Health", Company: "Star Health Insurance", Category: "Insurance", SubCategory: "Health"},
	{Pattern: "ACKO", Merchant: "Acko", Company: "Acko Technology", Category: "Insurance", SubCategory: "General"},
	{Pattern: "DIGIT INSURANCE", Merchant: "Digit", Company: "Go Digit General Insurance", Category: "Insurance", SubCategory: "General"},

	// Education
	{Pattern: "UDEMY", Merchant: "Udemy", Company: "Udemy Inc", Category: "Education", SubCategory: "Online Course"},
	{Pattern: "COURSERA", Merchant: "Coursera", Company: "Coursera Inc", Category: "Education", SubCategory: "Online Course"},
	{Pattern: "UNACADEMY", Merchant: "Unacademy", Company: "Sorting Hat Technologies", Category: "Education", SubCategory: "Online Course"},
	{Pattern: "BYJU", Merchant: "BYJU'S", Company: "Think & Learn", Category: "Education", SubCategory: "Online Course"},
	{Pattern: "UPGRAD", Merchant: "upGrad", Company: "upGrad Education", Category: "Education", SubCategory: "Online Course"},

	// Payments & Transfers (flag as payment gateway)
	{Pattern: "PHONEPE", Merchant: "PhonePe", Company: "PhonePe", Category: "Payments & Transfers", SubCategory: "UPI"},
	{Pattern: "RAZORPAY", Merchant: "Razorpay", Company: "Razorpay", Category: "Payments & Transfers", SubCategory: "Payment Gateway"},
	{Pattern: "PAYTM", Merchant: "Paytm", Company: "One97 Communications", Category: "Payments & Transfers", SubCategory: "Wallet"},
	{Pattern: "GOOGLEPAY", Merchant: "Google Pay", Company: "Google", Category: "Payments & Transfers", SubCategory: "UPI"},
	{Pattern: "CRED", Merchant: "CRED", Company: "Dreamplug Technologies", Category: "Payments & Transfers", SubCategory: "Bill Payment"},
	{Pattern: "MOBIKWIK", Merchant: "MobiKwik", Company: "One MobiKwik Systems", Category: "Payments & Transfers", SubCategory: "Wallet"},

	// Government & Tax
	{Pattern: "INCOME TAX", Merchant: "Income Tax Dept", Company: "Govt of India", Category: "Government & Tax", SubCategory: "Income Tax"},
	{Pattern: "TIN-NSDL", Merchant: "TIN-NSDL", Company: "NSDL", Category: "Government & Tax", SubCategory: "Income Tax"},
	{Pattern: "PASSPORT", Merchant: "Passport Seva", Company: "MEA", Category: "Government & Tax", SubCategory: "Passport"},
	{Pattern: "MCA PAYMENT", Merchant: "MCA", Company: "Ministry of Corporate Affairs", Category: "Government & Tax", SubCategory: "GST"},
}
