import React, {useEffect, useState} from 'react';
import './App.css';
import 'antd/dist/antd.css';
import MyGallery from "./pages/gallery";
import MyCalendar from "./pages/calendar";
import TreeFolder, {getBaseUrl} from "./pages/treeFolder";
import UploadFolder from "./pages/upload";
import {Layout, Menu, Switch} from 'antd';
import {HddFilled, PlusCircleOutlined} from "@ant-design/icons";
import {createBrowserHistory} from 'history';
import axios from "axios";
import Login from "./pages/login";

export const history = createBrowserHistory({
    basename: process.env.PUBLIC_URL
});

function checkAdminAccess(setCanAdmin,setShowPanel,askAuth = false,auth = {}){
    return axios({
        method: 'GET',
        url: getBaseUrl() + '/canAdmin',
        auth:auth
    }).then(d => {
        // If no access, ask again with basic auth
        if(!d.data.can && !askAuth) {
            setShowPanel(true)
        }else {
            setCanAdmin(d.data.can);
            return new Promise((ok,fail)=>{
                if(!d.data.can){
                    fail();
                }else{
                    ok();
                }
            })
        }
    });
}

function App() {
    const { Sider,Content } = Layout;

    const [collapsed,setCollapsed] = useState(false)
    const [showGallery,setShowGallery] = useState(true)

    const toggleCollapsed = () => {
        setCollapsed(!collapsed);
    };
    const [urlFolder,setUrlFolder] = useState({load:'',tags:''});
    // Used to refresh tree folder list
    const [update,setUpdate] = useState(false);
    const [currentFolder,setCurrentFolder] = useState('');
    const [titleGallery,setTitleGallery] = useState('');
    const [canAdmin,setCanAdmin] = useState(false);
    const [nbPhotos,setNbPhotos] = useState(0);
    const [showPanelLogin,setShowPanelLogin] = useState(false);

    const [isAddPanelVisible,setIsAddPanelVisible] = useState(false);
    const [isAddFolderPanelVisible,setIsAddFolderPanelVisible] = useState(false);

    useEffect(()=> {
        checkAdminAccess(setCanAdmin,setShowPanelLogin);
        axios({
            method: 'GET',
            url: getBaseUrl() + '/count',
        }).then(d => setNbPhotos(d.data));

    },[]);

    return (
        <Layout hasSider={true}>
            <Sider collapsible collapsed={collapsed} onCollapse={toggleCollapsed} width={300}>
                <Content style={{height:100+'%'}}>
                    <Menu theme={"dark"}>
                        <Menu.Item className={"logo"}>
                            <HddFilled/><span style={{marginLeft:10+'px'}}>Serveur photos - {nbPhotos}</span>
                        </Menu.Item>
                        {canAdmin?
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
                <MyGallery urlFolder={urlFolder} refresh={collapsed}
                           titleGallery={titleGallery}
                           canAdmin={canAdmin}
                           setCurrentFolder={setCurrentFolder}
                           update={update}
                           setUpdate={setUpdate}
                           setUrlFolder={setUrlFolder}
                           setIsAddFolderPanelVisible={setIsAddFolderPanelVisible}/>
                <UploadFolder setUpdate={setUpdate}
                              isAddPanelVisible={isAddPanelVisible}
                              setIsAddPanelVisible={setIsAddPanelVisible}/>
                {/*Panel to upload in a specific folder*/}

                <UploadFolder setUpdate={setUpdate}
                              isAddPanelVisible={isAddFolderPanelVisible}
                              setIsAddPanelVisible={setIsAddFolderPanelVisible}
                              defaultPath={currentFolder}
                              singleFolderDisplay={true}
                />
            </Layout>
            <Login checkAdminAccess={checkAdminAccess} setCanAdmin={setCanAdmin} setShowPanelLogin={setShowPanelLogin} showPanelLogin={showPanelLogin}/>
        </Layout>
    );
}

export default App;
