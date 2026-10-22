const form = document.getElementById("signupform");
const signup_api = "/api/login";
const challenge = "0000"

var url = location.href;
var login_state = {};
var signup_request = {
	username: "",
	password: "",
	login_code: "", // only used if LoginCode is enabled
	login_id: 0, // these  are only used if POW is enabled
	seed: "",
	nonce: "",
}

let timeoutUserStateId

async function send_user_json(user_json){
	const full_path = url + signup_api;
	const resp = await fetch(signup_api, {
		method: "POST", 
		headers: {
			"Content-Type": "application/json"
		},
		body: user_json,
	});
	if(!resp.ok){
		alert("Failed to send json")
	}
}



async function get_login_options(){
	const resp = await fetch(signup_api)
	if(!resp.ok){
		alert("Failed to get Login Options");
		return {}
	}
	return login_options = await resp.json();
}

async function sha256(message) {
  // 1. Encode the string into UTF-8 bytes
  const msgBuffer = new TextEncoder().encode(message);                    

  // 2. Hash the bytes using the built-in Web Crypto API
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);

  // 3. Convert the resulting ArrayBuffer into a readable Hex string
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
  
  return hashHex;
}

async function compute_hash(){
	var nonce = 0;
	while(true){
		var nonce_str = nonce.toString()
		var concat_str = login_state.PowSeed + nonce_str
		var hash_hex = await sha256(concat_str);
		var first_4_chars = hash_hex.slice(0,4);
		if(first_4_chars == challenge){
			signup_request.nonce = nonce_str;
			break;
		}
		nonce += 1;
	}
}

function create_access_code_box(){
	const submit_btn = form.querySelector('input[type="submit"]');
	
	//create label for login_code
	const login_code_lbl = document.createElement('label');
	login_code_lbl.setAttribute("for","login_code");
	login_code_lbl.textContent = "Login Code:"
	
	//create login_code_entry
	const login_code = document.createElement("input");
	login_code.type = "text";
	login_code.name = "login_code";
	login_code.placeholder = "login code";
	
	//create breaks
	const br1 = document.createElement("br");
	const br2 = document.createElement("br");
	const br3 = document.createElement("br");
	form.insertBefore(br1,submit_btn);
	form.insertBefore(login_code_lbl,submit_btn);
	form.insertBefore(br2,submit_btn);
	form.insertBefore(login_code,submit_btn);
	form.insertBefore(br3,submit_btn);
}

async function get_signup_result(){
	debugger;
	const id_str = login_state.LoginId.toString();
	const request_url = signup_api + "/" + id_str;
	const resp = await fetch(request_url, {
		method: "GET", 
	});
	if(!resp.ok){
		alert("Failed to send json");
		login_state = await get_login_options();
		start_user_state_timer();
	}
	const result = await resp.json();
	if(!result.login_ok){
		alert(result.err_msg)
		login_state = await get_login_options();
		start_user_state_timer();
	}else{
		alert("Login Successful!")
	}
}

async function handle_submit(event){
	debugger;
	event.preventDefault();
	const formData = new FormData(form); 
	const formContents = Object.fromEntries(formData);
	await compute_hash();
	signup_request.username = formContents.username;
	signup_request.password = formContents.password;
	if(login_state.RequiresLoginCode){
		signup_request.login_code = formContents.login_code;
	}
	signup_request.login_id = login_state.LoginId;
	signup_request.seed = login_state.PowSeed
	var signup_json = JSON.stringify(signup_request);
	await send_user_json(signup_json);
	await get_signup_result();
}


function start_user_state_timer() {

  clearTimeout(timeoutUserStateId); 
  
	timeoutUserStateId = setTimeout(async () => {
		debugger;
		login_state = await get_login_options();
		start_user_state_timer();
	}, 300000);
}

async function main(){
	debugger;
	login_state = await get_login_options();
	start_user_state_timer();
	if(login_state.RequiresLoginCode){
		create_access_code_box();
	}
	console.log("Setting submit hook");
	form.addEventListener('submit', handle_submit);
}

main()
