import axios from "axios";
import React, {useEffect, useState} from 'react'
import {Button, Col, Input, Modal, notification, Row, Select} from 'antd'
import {getBaseUrl} from "../treeFolder";

const {Option} = Select;

export default function SharePanel({path,showSharePanel,hide}) {
    let baseUrl = getBaseUrl();
    const [users,setUsers] = useState([]);
    const [name,setName] = useState('');
    const [userToRemove,setUserToRemove] = useState('');
    useEffect(()=>{
        if(showSharePanel){
            // Load shares for path
            axios({
                url:`${baseUrl}/share?path=${path}`,
                method:'GET'
            }).then(data=>setUsers(data.data != null ? data.data : []));
        }
    },[showSharePanel,path,baseUrl]);

    const add = ()=> {
        axios({
            url:`${baseUrl}/share?user=${name}&path=${path}`,
            method:'POST'
        }).then(()=>{
            setUsers(u=>[...u,name]);
            setName('');
            notification["success"]({message:'Succès',description:`Utilisateur ${name} ajouté au partage`})
        });
    };

    const remove  = ()=> {
        if(userToRemove === ''){
            return;
        }
        axios({
            url:`${baseUrl}/share?user=${userToRemove}&path=${path}`,
            method:'DELETE'
        }).then(()=>{
            setUsers(users.filter(u=>u !== userToRemove));
            setUserToRemove(users.length > 0 ? users[0]:'');
            notification["success"]({message:'Succès',description:`Utilisateur ${userToRemove} supprimé du partage`})
        });
    };

    return (
        <Modal
            title="Gestion des partages"
            visible={showSharePanel}
            okText={"Fermer"}
            onOk={hide}
            onCancel={hide}
            maskClosable={false}
            cancelButtonProps={{ style: { display: 'none' } }}
        >
            <Row>
                <Col span={4}>Partage(s)</Col>
                <Col span={12}>
                    <Select style={{width:100+'%'}} onChange={val=>setUserToRemove(val)} value={userToRemove}>
                        {users != null ? users.map(u=><Option value={u}>{u}</Option>):
                            <></>}
                    </Select>
                </Col>
                <Col span={6}><Button onClick={remove} style={{marginLeft:10}}>Supprimer</Button></Col>
            </Row>
            <Row>
                <Col span={4}>Nom</Col>
                <Col span={12}><Input style={{width:100+'%'}} onChange={e=>setName(e.target.value)} value={name}/></Col>
                <Col span={6}><Button onClick={add} style={{marginLeft:10}}>Ajouter</Button></Col>
            </Row>
        </Modal>
    )
}