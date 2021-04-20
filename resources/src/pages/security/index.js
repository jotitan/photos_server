import React,{useEffect,useState} from 'react';
import 'moment/locale/fr';
import axios from "axios";
import {getBaseUrl} from "../treeFolder";
import Login from "../login";


export default function ConnectPanel({setCanAccess}) {
    // Get authentication configuration
    const [basic,setBasic] = useState(false);

    useEffect(()=>{
        axios({
            method:'GET',
            url:getBaseUrl() + '/security/config'
        }).then(d=>{
            switch(d.data.name){
                case "basic":setBasic(true);break;
                case "oauth2":
                    // redirect
                    window.location.href=d.data.url;
                    break;
                default:console.log("unknown case");
            }
        })
    },[]);

    return (
        basic ? <Login setCanAccess={setCanAccess} />:
            <></>
    );
}