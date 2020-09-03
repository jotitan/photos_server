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
import ConnectPanel from "./pages/security";

export const history = createBrowserHistory({
    basename: process.env.PUBLIC_URL
});

function checkReadAccess(){
    return axios({
        method: 'GET',
        url: getBaseUrl() + '/canAccess',
    })
}

function checkAdminAccess(setCanAdmin){
    return axios({
        method: 'GET',
        url: getBaseUrl() + '/canAdmin',
    }).then(() =>
        // If 200, can admin, otherwise, 403
        setCanAdmin(true)
    );
}

const detectParameters = ()=> {
    return window.location.search.indexOf("?")!==-1 ?
        window.location.search
            .substr(1).split("&")
            .map(param=>{
                let subs = param.split("=");
                return {name:subs[0],value:subs[1]}
            }):[];
};

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
    const [hideAll,setHideAll] = useState(true);
    const [nbPhotos,setNbPhotos] = useState(0);

    const [isAddPanelVisible,setIsAddPanelVisible] = useState(false);
    const [isAddFolderPanelVisible,setIsAddFolderPanelVisible] = useState(false);

    const [canAccess,setCanAccess] = useState(false);
    // First load
    useEffect(()=> {
        let params = detectParameters();
        // If parameters in command line, connexion by oauth2, try to connect
        if(params.length > 0){
            axios({
                method:'POST',
                url: getBaseUrl() + '/connect',
                data:params
            }).then(()=>checkReadAccess()
                .then(()=>setCanAccess(true))
                .finally(()=>{
                    setHideAll(false);
                    console.log(window.location.href,window.location.search,window.location);
                    console.log(window.location.href.replace(window.location.search,''));
                    history.push(window.location.href.replace(window.location.search,''));
                }))
        }else {
            checkReadAccess()
                .then(() => setCanAccess(true))
                .finally(()=>setHideAll(false));
        }
    },[]);

    useEffect(()=> {
        if(canAccess) {
            checkAdminAccess(setCanAdmin);
            axios({
                method: 'GET',
                url: getBaseUrl() + '/count',
            }).then(d => setNbPhotos(d.data));
        }
    },[canAccess]);

    return (
        // Hide during check access
        hideAll ? <></>:
            canAccess?
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
                </Layout>:<>
                    <ConnectPanel setCanAccess={setCanAccess}/>
                </>);
}

export default App;
