import {AuthenticationService} from "./auth";
const server = (): void=>{
    console.log("Initializing...")

    // Register Services
    const authService = new AuthenticationService()

    authService.run()
}
server()
export default server