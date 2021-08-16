import {Codec, connect, ConnectionOptions, NatsConnection, StringCodec} from "nats"
import * as jwt from "jsonwebtoken"
import * as cookie from "cookie"

class AuthenticationService {
    private readonly nats: Promise<NatsConnection>
    private readonly opts: ConnectionOptions
    private readonly natsSubject: string
    private readonly cookieName: string

    constructor() {
        // Subject to listen for events on
        this.natsSubject = "auth.auth.jwtHeader"
        // this.cookieName = "development-access"
        this.cookieName = "access-token"
        // Nats connection options
        // Specify one or more servers to connect to
        this.opts = {
            servers: "nats://localhost:4222"
        }

        // Issue connection to nats server
        this.nats = connect(this.opts)


    }

    private static generateID(cid:string):string{
        return `conn.${cid}.token`
    }

    async run(): Promise<void> {
        const nats = await this.nats
        const sc:Codec<string> = StringCodec()
        console.log("Running...")
        const subscription = nats.subscribe(this.natsSubject)

        // Process each message yeilded from subscription iterator
        for await (const message of subscription) {

            // Grab the reply address and data from the message
            const { reply, data} = message
            console.log("[Auth 2.0] Received new message", reply)


            // Don't process malformed requests
            // reply is required in our case
            // so we skip to next in iterator
            if(reply === undefined){
                continue
            }

            // Grab the header and connection ID from the data packet
            const {header, cid} = JSON.parse(sc.decode(data))

            // Grab all cookies from the header
            const cookies = header["Cookie"] && cookie.parse(header["Cookie"][0])

            // Grab our token from the cookies
            const token = jwt.decode(cookies[this.cookieName])

            // Ensure the validity of the token, ideally check the signature
            //TODO: Call Keycloak, passing the token

            // Generate the connection string
            const connectionID = AuthenticationService.generateID(cid)

            /**
             *  This is the fun part.
             *
             *  Instrumentation:
             *  1. $ nats sub >
             *  - watches all nats messages being sent to the broker
             *  2. append --trace to Resgate docker run command
             *
             *  Steps to reproduce:
             *
             *  1. Bring up auth service (../jwt-authentication/authService.js) (yarn start)
             *  2. Comment out the two publish lines below
             *  3. Bring up this service (yarn run dev)
             *  4. Navigate to http://localhost:8084
             *  5. Login
             *  6. Go back
             *  7. Refresh the page
             *  8. Observe the terminal output of both services
             *
             *  So the other service is the one handling these requests,
             *  it works as expected the browser displays hello world.
             *
             *  Presumably we are doing the exact same thing in both services,
             *  as show by the terminal output...right?
             *
             *  Let's make sure:
             *  1. Comment out the publish lines in the other service
             *  2. Uncomment the lines in this service
             *  3. Refresh the browser (do it twice, it doesn't always work first try)
             *  4. Observe how the request fails yet we have the "same" thing being sent as the other service.
             *
             *  ¯\_(ツ)_/¯
             *
             *  ie:
             *
             Original Service:

             [Auth] Received new message _INBOX.7Ap64yZ6OjkInVcCNbwmv2
             [Auth] Response 1:  {"token":{"foo":"bar","iat":1629071958}}
             [Auth] Response 2:  {"result":null}
             [Auth] Processed message
             [Example] Token is:  { foo: 'bar', iat: 1629071958 }

             This Service:

             [Auth 2.0] Received new message _INBOX.7Ap64yZ6OjkInVcCNbwmv2
             [Auth 2.0] Repsponse 1:  {"token":{"foo":"bar","iat":1629071958}}
             [Auth 2.0] Response 2:  {"result":null}
             [Auth 2.0] Processed message
             ... [Example] Token is:  null


             Yields:
             {"error":{"code":"system.accessDenied","message":"Access denied"},"id":3}

             */

            // Generate the payload to go to ResGate
            // token -> json() -> string -> encode() -> uint8[]
            const tokenPayload = sc.encode( JSON.stringify({token}))
            console.log("[Auth 2.0] Repsponse 1: ", sc.decode(tokenPayload))
            // Pass the token to Resgate to authenticate the client
            // nats.publish(connectionID,tokenPayload)

            // Respond with result:null to let the client know that they are authenticated
            const okayPayload = sc.encode(JSON.stringify({result:null}))
            console.log("[Auth 2.0] Response 2: ", sc.decode( okayPayload))
            // nats.publish(reply,okayPayload)
            console.log("[Auth 2.0] Processed message")
        }
    }
}

export {
    AuthenticationService
}