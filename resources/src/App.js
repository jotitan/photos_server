import React, {useEffect, useState} from 'react';
import './App.css';
import 'antd/dist/antd.css';
import MyGallery from "./pages/gallery";
import MyCalendar from "./pages/calendar";
import TreeFolder,{getBaseUrl} from "./pages/treeFolder";
import UploadFolder from "./pages/upload";
import {Layout, Menu, Switch} from 'antd';
import {HddFilled,PlusCircleOutlined} from "@ant-design/icons";
import {createBrowserHistory} from 'history';
import axios from "axios";

export const history = createBrowserHistory({
    basename: process.env.PUBLIC_URL
});

function App() {
    const { Sider,Content } = Layout;

    const [collapsed,setCollapsed] = useState(false)
    const [showGallery,setShowGallery] = useState(true)

    const toggleCollapsed = () => {
        setCollapsed(!collapsed);
    };
    const [urlFolder,setUrlFolder] = useState({load:'',tags:''});
    const [update,setUpdate] = useState(false);
    const [titleGallery,setTitleGallery] = useState('');
    const [canDelete,setCanDelete] = useState(false);

    const [isAddPanelVisible,setIsAddPanelVisible] = useState(false);

    useEffect(()=> {
        axios({
            method: 'GET',
            url: getBaseUrl() + '/canDelete',
        }).then(d => setCanDelete(d.data.can))
    },[]);

    return (
        <Layout hasSider={true}>
            <Sider collapsible collapsed={collapsed} onCollapse={toggleCollapsed} width={300}>
                <Content style={{height:100+'%'}}>
                    <Menu theme={"dark"}>
                        <Menu.Item className={"logo"}>
                            <HddFilled/><span style={{marginLeft:10+'px'}}>Serveur photos</span>
                        </Menu.Item>
                        {canDelete ?
                        <Menu.Item className={"add-folder-text"} onClick={()=>setIsAddPanelVisible(true)}>
                            <PlusCircleOutlined /> <span>Ajouter des photos</span>
                        </Menu.Item>:<></>}
                    </Menu>
                    {!collapsed ?
                        <div style={{color:'white',padding:10+'px'}}>
                            <span style={{paddingRight:10+'px'}}> Dossiers</span>
                            <Switch onChange={isCalendar=>setShowGallery(!isCalendar)} className={"switch-selection"}/>
                            <span style={{paddingLeft:10+'px'}}> Calendrier</span>
                        </div>:<></>}

                    {!collapsed ? showGallery ?
                        <TreeFolder setUrlFolder={setUrlFolder} setTitleGallery={setTitleGallery} update={update}/>:
                        <div style={{width:300+'px'}}>
                            <MyCalendar setUrlFolder={setUrlFolder} setTitleGallery={setTitleGallery} update={update} />
                        </div>:
                        <></>}
                </Content>
            </Sider>
            <Layout>
                <MyGallery urlFolder={urlFolder} refresh={collapsed} titleGallery={titleGallery} canDelete={canDelete}/>
                <UploadFolder setUpdate={setUpdate} isAddPanelVisible={isAddPanelVisible} setIsAddPanelVisible={setIsAddPanelVisible}/>
            </Layout>
        </Layout>
    );
}

export default App;
