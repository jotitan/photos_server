import React, {useState} from 'react';
import 'moment/locale/fr';
import {Col, Input, Modal, Row} from 'antd';
import axios from "axios";
import {getBaseUrl} from "../treeFolder";

export default function Login({setCanAccess}) {
    const [username,setUsername] = useState('');
    const [password,setPassword] = useState('');
    const [message,setMessage] = useState('');
    const [showPanel,setShowPanel] = useState(true);

    const basicConnect = ()=>{
        return axios({
            method: 'GET',
            url: getBaseUrl() + '/connect',
            auth:{username:username,password:password}
        }).then(d => {
            // If no access, ask again with basic auth
            setMessage("");
            setShowPanel(true);
            setCanAccess(true);
        }).catch(()=>{
            setMessage("Impossible de se connecter, mauvais login / mot de passe");
            setCanAccess(false);
        });
    };

    return (
        <Modal
            title="Login"
            className={"upload-photos-modal"}
            visible={showPanel}
            onOk={()=>basicConnect()}
            onCancel={()=>setShowPanel(false)}
            maskClosable={false}
            okText={"Se connecter"}
            cancelText={"Lecture seule"}
        >
            {message !== '' ? <Row style={{color:'red',fontWeight:'bold'}}>{message}</Row>:''}
            <Row style={{marginBottom:10}}>
                <Col flex={"100px"} style={{paddingLeft:10}}>Login</Col>
                <Col><Input onChange={e=>setUsername(e.target.value)} style={{width:'180px'}}/></Col>
            </Row>
            <Row>
                <Col flex={"100px"} style={{paddingLeft:10}}>Mot de passe</Col>
                <Col><Input.Password onChange={e=>setPassword(e.target.value)} style={{width:'180px'}}/></Col>
            </Row>
        </Modal>);

}