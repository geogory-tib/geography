const form = document.getElementById("signupform");
const signup_api = "/api/login";
var url = location.href;



async function send_user_json(user_json){
	const full_path = url + signup_api;
	const resp = await fetch(signup_api, {
		method: "POST", 
		headers: {
			"Content-Type": "application/json"
		},
		body: user_json,
	});
}

form.addEventListener('submit', async (e) => {
	e.preventDefault();
	debugger;
	const formData = new FormData(form); 
	const formContents = Object.fromEntries(formData);
	var user_json = JSON.stringify(formContents); 
	await send_user_json(user_json);
});
