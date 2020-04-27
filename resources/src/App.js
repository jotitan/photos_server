import React, {useState} from 'react';
import './App.css';
import 'antd/dist/antd.css';
import MyGallery from "./pages/gallery";
import MyCalendar from "./pages/calendar";
import TreeFolder from "./pages/treeFolder";
import {Layout, Menu,Switch} from 'antd';
import {HddFilled} from "@ant-design/icons";
import { createBrowserHistory } from 'history';

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
    const [urlFolder,setUrlFolder] = useState('');
    const [titleGallery,setTitleGallery] = useState('');
    return (
        <Layout hasSider={true}>
            <Sider collapsible collapsed={collapsed} onCollapse={toggleCollapsed} width={300}>
                <Content style={{height:100+'%'}}>
                    <Menu theme={"dark"}>
                        <Menu.Item className={"logo"}>
                            <HddFilled/><span style={{marginLeft:10+'px'}}>Serveur photos</span>
                        </Menu.Item>
                    </Menu>
                    {!collapsed ?
                    <div style={{color:'white',padding:10+'px'}}>
                        <Switch onChange={isCalendar=>setShowGallery(!isCalendar)}/>
                        <span style={{paddingLeft:10+'px'}}>Dossiers / Calendrier</span>
                    </div>:<></>}

                    {!collapsed ? showGallery ?
                        <TreeFolder setUrlFolder={setUrlFolder} setTitleGallery={setTitleGallery}/>:
                        <div style={{width:300+'px'}}><MyCalendar setUrlFolder={setUrlFolder} setTitleGallery={setTitleGallery}/></div>:
                        <></>}

                </Content>
            </Sider>
            <Layout>
                <MyGallery urlFolder={urlFolder} refresh={collapsed} titleGallery={titleGallery}/>
            </Layout>
        </Layout>
    );
}

export default App;
