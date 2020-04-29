import React, {useEffect, useState} from 'react'
import {Tree} from 'antd'
import axios from "axios";
import useLocalStorage from "../../services/local-storage.hook";


export const getBaseUrlHref = ()=>{
    return getBaseUrl(window.location.href)
}

export const getBaseUrl = (defaultValue=window.location.origin)=>{
    switch (window.location.hostname) {
        case 'localhost':
            return 'http://localhost:9006';
        default : return defaultValue;
    }
}

const sortByName = (a,b)=>a.Name === b.name ? 0:a.Name < b.Name ? -1:1;

const adapt = node => {
    let data = {title:node.Name.replace(/_/g," "),key:getBaseUrl() + node.Link}
    data.hasImages = node.HasImages;

    if(node.Children != null && node.Children.length > 0){
        data.children = node.Children.sort(sortByName).map(nc=>adapt(nc));
    }else{
        data.isLeaf=true
    }
    return data;
}

export default function TreeFolder({setUrlFolder,setTitleGallery,update}) {
    const [tree,setTree] = useState([]);
    const { DirectoryTree } = Tree;
    const [height,setHeight] = useState(window.innerHeight-95);
    const [expandables,setExpandables] = useLocalStorage("expandables",[])
    useEffect(()=>{
        axios({
            method:'GET',
            url:getBaseUrl() + '/rootFolders',
        }).then(d=>{
            setTree([adapt(d.data)]);
        })
    },[update]);

    const onSelect = (e,f)=>{
        setTitleGallery('');
        if(f.node.children == null || f.node.children.length === 0) {
            setUrlFolder(e[0])
        }else{
            // Case when folder has sub folders but also images
            if(f.node.hasImages){
                setUrlFolder(e[0])
            }
        }
    };

    window.addEventListener('resize', ()=>setHeight(window.innerHeight-95));
    const onExpand = values=>{
        setExpandables(values)
    }
    return(
        tree.length === 0 ? '' :
            <DirectoryTree
                onSelect={onSelect}
                treeData={tree}
                height={height}
                autoExpandParent={false}
                expandedKeys={expandables}
                onExpand={onExpand}
                virtual={true}
                style={{
                    fontSize: 12 + 'px',
                    width: 300 + 'px',
                    overflow: 'auto',
                    backgroundColor: '#001529',
                    color: '#999'
                }}
            />
    )
}